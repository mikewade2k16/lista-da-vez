package customerdata

import "testing"

func TestModuleCatalog(t *testing.T) {
	t.Parallel()
	module := New()
	if module.ID() != "customer_data" {
		t.Fatalf("unexpected module id %q", module.ID())
	}
	metadata := module.Metadata()
	if metadata.SchemaName != "customer_data" || metadata.SortOrder != 44 {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if len(metadata.RequiresModules) != 1 || metadata.RequiresModules[0] != "core" {
		t.Fatalf("customer data must require core: %+v", metadata.RequiresModules)
	}
	permissions := map[string]bool{}
	for _, permission := range module.Permissions() {
		permissions[permission.Key] = true
	}
	for _, key := range []string{
		"customer_data.subjects.view",
		"customer_data.relationships.manage",
		"customer_data.identities.manage",
		"customer_data.offline_interactions.manage",
		"customer_data.consents.manage",
		"customer_data.merge.manage",
		"customer_data.segments.publish",
		"customer_data.capabilities.manage",
	} {
		if !permissions[key] {
			t.Fatalf("missing permission %s", key)
		}
	}
	var adminFound bool
	for _, role := range module.RoleTemplates() {
		if role.ID != "customer_data.admin" {
			continue
		}
		adminFound = true
		var managesControlState bool
		for _, permission := range role.Permissions {
			if permission == "customer_data.capabilities.manage" {
				managesControlState = true
			}
		}
		if !managesControlState {
			t.Fatal("customer data admin must manage control state")
		}
	}
	if !adminFound {
		t.Fatal("missing customer data admin role template")
	}
}
