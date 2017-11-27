package vault

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
)

func TestAccDatabaseSecretBackendCredentials_basic(t *testing.T) {
	connURL := os.Getenv("POSTGRES_URL")
	if connURL == "" {
		t.Skip("POSTGRES_URL not set")
	}
	backend := acctest.RandomWithPrefix("tf-test-db")
	name := acctest.RandomWithPrefix("role")
	dbName := acctest.RandomWithPrefix("db")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseSecretBackendCredentialsConfig_basic(name, dbName, backend, connURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("vault_database_secret_backend_credentials.creds", "name", name),
					resource.TestCheckResourceAttr("vault_database_secret_backend_credentials.creds", "backend", backend),
					resource.TestCheckResourceAttrSet("vault_database_secret_backend_credentials.creds", "username"),
					resource.TestCheckResourceAttrSet("vault_database_secret_backend_credentials.creds", "password"),
					resource.TestCheckResourceAttrSet("vault_database_secret_backend_credentials.creds", "lease_renewable"),
					resource.TestCheckResourceAttrSet("vault_database_secret_backend_credentials.creds", "lease_duration"),
					resource.TestCheckResourceAttrSet("vault_database_secret_backend_credentials.creds", "lease_started"),
				),
			},
		},
	})
}

func testAccDatabaseSecretBackendCredentialsConfig_basic(name, db, path, connURL string) string {
	return fmt.Sprintf(`
resource "vault_mount" "db" {
  path = "%s"
  type = "database"
}

resource "vault_database_secret_backend_connection" "test" {
  backend = "${vault_mount.db.path}"
  name = "%s"
  allowed_roles = ["%s"]

  postgresql {
	  connection_url = "%s"
  }
}

resource "vault_database_secret_backend_role" "test" {
  backend = "${vault_mount.db.path}"
  db_name = "${vault_database_secret_backend_connection.test.name}"
  name = "%s"
  default_ttl = 3600
  max_ttl = 7200
  creation_statements = "CREATE ROLE \"{{name}}\" WITH PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';"
}

resource "vault_database_secret_backend_credentials" "creds" {
  backend = "${vault_database_secret_backend_role.test.backend}"
  name = "${vault_database_secret_backend_role.test.name}"
}
`, path, db, name, connURL, name)
}
