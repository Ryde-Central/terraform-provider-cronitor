package provider

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// The tests in this package run real terraform/tofu plan/apply cycles against
// an in-memory fake of the Cronitor API (see fake_api_test.go). If terraform is
// not on PATH but tofu is, tofu is used.
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		if _, err := exec.LookPath("terraform"); err != nil {
			if tofu, err := exec.LookPath("tofu"); err == nil {
				os.Setenv("TF_ACC_TERRAFORM_PATH", tofu)
				os.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
			}
		}
	}
	os.Exit(m.Run())
}

var testProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"cronitor": providerserver.NewProtocol6WithError(New("test")()),
}

func providerConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "cronitor" {
  api_key = "test-key"
  api_url = "%s/api"
}
`, baseURL)
}
