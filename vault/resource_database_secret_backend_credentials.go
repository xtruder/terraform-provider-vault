package vault

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/vault/api"
)

func databaseSecretBackendCredentialsResource() *schema.Resource {
	return &schema.Resource{
		Create: databaseSecretBackendCredentialsCreate,
		Read:   databaseSecretBackendCredentialsRead,
		Delete: databaseSecretBackendCredentialsDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the role to get credentials for.",
			},
			"backend": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The path of the Database Secret Backend the role belongs to.",
			},
			"username": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The generated username.",
			},
			"password": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The generated password.",
			},
			"lease_renewable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the lease can be renewed or not.",
			},
			"lease_duration": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of seconds the lease is valid for.",
			},
			"lease_started": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "RFC 3339 timestamp representing the start of the lease.",
			},
		},
	}
}

func databaseSecretBackendCredentialsCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	backend := d.Get("backend").(string)
	name := d.Get("name").(string)

	path := databaseSecretBackendCredentialsPath(backend, name)

	log.Printf("[DEBUG] Reading credentials for role %q from database backend %q", name, backend)
	secret, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("Error creating role %q for backend %q: %s", name, backend, err)
	}
	log.Printf("[DEBUG] Created role %q on AWS backend %q", name, backend)

	d.SetId(secret.LeaseID)
	d.Set("lease_started", time.Now().Format(time.RFC3339))
	d.Set("name", name)
	d.Set("backend", backend)
	d.Set("username", secret.Data["username"])
	d.Set("password", secret.Data["password"])
	d.Set("lease_renewable", secret.Renewable)
	d.Set("lease_duration", secret.LeaseDuration)
	return nil
}

func databaseSecretBackendCredentialsRead(d *schema.ResourceData, meta interface{}) error {
	if !leaseExpiringSoon(d) {
		return nil
	}
	log.Printf("[DEBUG] Credentials expiring soon, obtaining new credentials.")
	return databaseSecretBackendCredentialsCreate(d, meta)
}

func databaseSecretBackendCredentialsDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	log.Printf("[DEBUG] Revoking credentials %q", d.Id())
	err := client.Sys().Revoke(d.Id())
	if err != nil {
		return fmt.Errorf("Error revoking credentials %q: %s", d.Id(), err)
	}
	return nil
}

func databaseSecretBackendCredentialsPath(backend, name string) string {
	return strings.Trim(backend, "/") + "/creds/" + strings.Trim(name, "/")
}

func leaseExpiringSoon(d *schema.ResourceData) bool {
	startedStr := d.Get("lease_started").(string)
	duration := d.Get("lease_duration").(int)
	if startedStr == "" {
		return false
	}
	started, err := time.Parse(time.RFC3339, startedStr)
	if err != nil {
		log.Printf("[DEBUG] lease_started %q for %q is an invalid value, removing: %s", startedStr, d.Id(), err)
		d.Set("lease_started", "")
		return false
	}
	// whether the time the lease started plus the number of seconds specified in the duration
	// plus five minutes of buffer is before the current time or not. If it is, we don't need to
	// renew just yet.
	if started.Add(time.Second * time.Duration(duration)).Add(time.Minute * 5).Before(time.Now()) {
		return false
	}
	// if the lease duration expired more than five minutes ago, we can't renew anyways, so don't
	// bother even trying.
	if started.Add(time.Second * time.Duration(duration)).After(time.Now().Add(time.Minute * -5)) {
		return false
	}

	// the lease will expire in the next five minutes, or expired less than five minutes ago, in
	// which case renewing is worth a shot
	return true
}
