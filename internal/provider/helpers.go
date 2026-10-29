package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

// clientFromProviderData extracts the API client configured by the provider.
// It returns nil (without error) before the provider has been configured.
func clientFromProviderData(data any, diags *diag.Diagnostics) *client.Client {
	if data == nil {
		return nil
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", data))
		return nil
	}
	return c
}

// baseResource holds the API client shared by every resource.
type baseResource struct {
	client *client.Client
}

func (r *baseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// baseDataSource holds the API client shared by every data source.
type baseDataSource struct {
	client *client.Client
}

func (d *baseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

// stringOrNull maps the API's "unset" (empty) string to null.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func stringPtrOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return stringOrNull(*s)
}

func int64PtrOrNull(i *client.FlexInt) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*i))
}

// known reports whether a value was set in configuration (or is known in plan).
func known(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// nullableString returns the value as a JSON-able string, or nil for null.
func nullableString(v types.String) any {
	if !known(v) {
		return nil
	}
	return v.ValueString()
}

// setValue builds a set of strings from API values. An empty API value becomes
// null unless the prior value was an explicitly empty set, so that omitting an
// optional attribute does not produce a diff against the API's []. Repeated
// values are dropped: the API can list one more than once (a monitor's
// environments do), and a set can't hold duplicates.
func setValue(vals []string, prior types.Set) types.Set {
	if len(vals) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.SetNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(vals))
	seen := make(map[string]bool, len(vals))
	for _, v := range vals {
		if seen[v] {
			continue
		}
		seen[v] = true
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

// listValue is setValue for ordered lists.
func listValue(vals []string, prior types.List) types.List {
	if len(vals) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, 0, len(vals))
	for _, v := range vals {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

func mapValue(vals map[string]string, prior types.Map) types.Map {
	if len(vals) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.MapNull(types.StringType)
	}
	elems := make(map[string]attr.Value, len(vals))
	for k, v := range vals {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}

// stringsFrom converts a set or list of strings to a non-nil slice.
func stringsFrom(ctx context.Context, v interface {
	attr.Value
	ElementsAs(context.Context, any, bool) diag.Diagnostics
}, diags *diag.Diagnostics) []string {
	out := []string{}
	if !known(v) {
		return out
	}
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

func stringMapFrom(ctx context.Context, v types.Map, diags *diag.Diagnostics) map[string]string {
	out := map[string]string{}
	if !known(v) {
		return out
	}
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// sameInstant keeps a prior RFC 3339 value when the API returns the same
// instant in a different textual form (e.g. "+00:00" instead of "Z").
func sameInstant(apiValue string, prior types.String) types.String {
	if known(prior) {
		a, errA := time.Parse(time.RFC3339, apiValue)
		b, errB := time.Parse(time.RFC3339, prior.ValueString())
		if errA == nil && errB == nil && a.Equal(b) {
			return prior
		}
	}
	return stringOrNull(apiValue)
}

var intervalRe = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*([a-zA-Z]*)\s*$`)

// intervalSeconds parses Cronitor interval expressions such as "8 hours",
// "30 min" or "1h". A bare number is interpreted as hours, matching the API's
// handling of realert_interval.
func intervalSeconds(s string) (float64, bool) {
	m := intervalRe.FindStringSubmatch(strings.TrimPrefix(strings.TrimSpace(strings.ToLower(s)), "every "))
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	var unit float64
	switch m[2] {
	case "s", "sec", "secs", "second", "seconds":
		unit = 1
	case "m", "min", "mins", "minute", "minutes":
		unit = 60
	case "", "h", "hr", "hrs", "hour", "hours":
		unit = 3600
	case "d", "day", "days":
		unit = 86400
	case "w", "week", "weeks":
		unit = 604800
	default:
		return 0, false
	}
	return n * unit, true
}

// sameInterval keeps a prior interval expression when the API returns an
// equivalent one in its own canonical form.
func sameInterval(apiValue string, prior types.String) types.String {
	if known(prior) {
		a, okA := intervalSeconds(apiValue)
		b, okB := intervalSeconds(prior.ValueString())
		if okA && okB && a == b {
			return prior
		}
	}
	return stringOrNull(apiValue)
}
