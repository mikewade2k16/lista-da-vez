package core

import "testing"

func TestValidateModuleDependencyChange(t *testing.T) {
	current := []AccountModuleView{
		{ModuleID: "customer_data", Enabled: false},
		{ModuleID: "customer_intelligence", Enabled: false},
	}

	if err := validateModuleDependencyChange(current, AdminSetModulesInput{
		Enable: []string{"customer_intelligence"},
	}); err == nil {
		t.Fatal("customer_intelligence sem customer_data deveria ser rejeitado")
	}

	if err := validateModuleDependencyChange(current, AdminSetModulesInput{
		Enable: []string{"customer_data", "customer_intelligence"},
	}); err != nil {
		t.Fatalf("habilitacao atomica das dependencias falhou: %v", err)
	}

	enabled := []AccountModuleView{
		{ModuleID: "customer_data", Enabled: true},
		{ModuleID: "customer_intelligence", Enabled: true},
	}
	if err := validateModuleDependencyChange(enabled, AdminSetModulesInput{
		Disable: []string{"customer_data"},
	}); err == nil {
		t.Fatal("desabilitar dependencia em uso deveria ser rejeitado")
	}

	if err := validateModuleDependencyChange(enabled, AdminSetModulesInput{
		Disable: []string{"customer_intelligence", "customer_data"},
	}); err != nil {
		t.Fatalf("desabilitacao conjunta deveria ser aceita: %v", err)
	}
}

func TestValidateModuleDependencyChangeRejectsConflictingInput(t *testing.T) {
	err := validateModuleDependencyChange(nil, AdminSetModulesInput{
		Enable:  []string{"customer_data"},
		Disable: []string{"customer_data"},
	})
	if err == nil {
		t.Fatal("mesmo modulo em enable/disable deveria falhar")
	}
}
