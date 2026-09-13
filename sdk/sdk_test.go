package sdk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"awo.so/awo/def"
	"awo.so/awo/sdk"
)

// --- EntityBuilder ---

func TestSystem_Build_IsSystem(t *testing.T) {
	d := sdk.System("fin_invoice", "fin", "Invoice", "Invoices").Build()
	assert.True(t, d.IsSystem())
	assert.Equal(t, "fin_invoice", d.EntityName())
	assert.Equal(t, "fin", d.EntityModule())
	assert.Equal(t, "Invoice", d.EntityLabel())
	assert.Equal(t, "Invoices", d.EntityLabelPlural())
}

func TestCustom_Build_IsNotSystem(t *testing.T) {
	d := sdk.Custom("crm_note", "crm", "Note", "Notes").Build()
	assert.False(t, d.IsSystem())
}

func TestEntityBuilder_WithFields(t *testing.T) {
	d := sdk.System("mod_item", "mod", "Item", "Items").
		Field(sdk.Data("name").Required().Searchable().Build()).
		Field(sdk.Int("quantity").Build()).
		Build()

	fields := d.EntityFields()
	require.Len(t, fields, 2)
	assert.Equal(t, "name", fields[0].Name)
	assert.True(t, fields[0].Required)
	assert.True(t, fields[0].Searchable)
	assert.Equal(t, "quantity", fields[1].Name)
	assert.Equal(t, def.FieldTypeInt, fields[1].Type)
}

func TestEntityBuilder_WithPermissions(t *testing.T) {
	d := sdk.System("mod_item", "mod", "Item", "Items").
		Permissions(sdk.AdminOnly()).
		Build()
	assert.Equal(t, []string{"role:platform-admin", "role:tenant.admin"}, d.EntityPermissions().Create)
}

func TestEntityBuilder_WithTrigger(t *testing.T) {
	d := sdk.System("mod_item", "mod", "Item", "Items").
		Trigger(sdk.OnCreate("ItemCreatedWorkflow", "mod.item.create").Build()).
		Build()
	triggers := d.EntityWorkflowTriggers()
	require.Len(t, triggers, 1)
	assert.Equal(t, def.EventOnCreate, triggers[0].On)
	assert.Equal(t, "ItemCreatedWorkflow", triggers[0].WorkflowFn)
}

// --- FieldBuilder ---

func TestData_Defaults(t *testing.T) {
	f := sdk.Data("name").Build()
	assert.Equal(t, "name", f.Name)
	assert.Equal(t, def.FieldTypeData, f.Type)
	assert.False(t, f.Required)
}

func TestData_AllModifiers(t *testing.T) {
	f := sdk.Data("email").
		Label("Email Address").
		Description("Primary email").
		Required().Unique().Immutable().Sensitive().Searchable().
		MaxLen(254).
		Default("noreply@example.com").
		Build()

	assert.Equal(t, "Email Address", f.Label)
	assert.Equal(t, "Primary email", f.Description)
	assert.True(t, f.Required)
	assert.True(t, f.Unique)
	assert.True(t, f.Immutable)
	assert.True(t, f.Sensitive)
	assert.True(t, f.Searchable)
	assert.Equal(t, 254, f.MaxLen)
	assert.Equal(t, "noreply@example.com", f.Default())
}

func TestSelect_Options(t *testing.T) {
	f := sdk.Select("status", "draft", "active", "archived").Build()
	assert.Equal(t, def.FieldTypeSelect, f.Type)
	assert.Equal(t, []string{"draft", "active", "archived"}, f.Options)
}

func TestLink_Target(t *testing.T) {
	f := sdk.Link("customer_id", "crm_customer").Build()
	assert.Equal(t, def.FieldTypeLink, f.Type)
	assert.Equal(t, "crm_customer", f.LinkTarget)
}

func TestNamingSeries_Series(t *testing.T) {
	f := sdk.NamingSeries("number", "INV-{YYYY}-{SEQ:5}").Build()
	assert.Equal(t, def.FieldTypeNamingSeries, f.Type)
	assert.Equal(t, "INV-{YYYY}-{SEQ:5}", f.Series)
}

func TestEmail_MaxLen(t *testing.T) {
	f := sdk.Email("email").Build()
	assert.Equal(t, 254, f.MaxLen)
}

func TestHidden_ReadOnly(t *testing.T) {
	h := sdk.Data("internal").Hidden().Build()
	ro := sdk.Data("locked").ReadOnly().Build()
	assert.True(t, h.Hidden)
	assert.True(t, ro.ReadOnly)
}

func TestCurrency(t *testing.T) {
	f := sdk.Currency("total_kes").Build()
	assert.Equal(t, def.FieldTypeCurrency, f.Type)
}

// --- Permissions ---

func TestAdminOnly(t *testing.T) {
	p := sdk.AdminOnly()
	assert.Contains(t, p.Create, "role:platform-admin")
	assert.Contains(t, p.Create, "role:tenant.admin")
	assert.NotContains(t, p.Read, "role:tenant.user")
}

func TestTenantScoped(t *testing.T) {
	p := sdk.TenantScoped()
	assert.Contains(t, p.Read, "role:tenant.user")
	assert.NotContains(t, p.Delete, "role:tenant.user")
}

func TestFullAccess(t *testing.T) {
	p := sdk.FullAccess()
	assert.Contains(t, p.Create, "role:tenant.user")
	assert.NotContains(t, p.Delete, "role:tenant.user")
}

func TestReadOnly(t *testing.T) {
	p := sdk.ReadOnly()
	assert.Empty(t, p.Create)
	assert.Empty(t, p.Write)
	assert.Empty(t, p.Delete)
	assert.NotEmpty(t, p.Read)
}

func TestPlatformOnly(t *testing.T) {
	p := sdk.PlatformOnly()
	assert.Equal(t, []string{"role:platform-admin"}, p.Create)
}

func TestWithPolicy(t *testing.T) {
	base := sdk.AdminOnly()
	assert.Nil(t, base.Policy)
	withPol := sdk.WithPolicy(base, def.NoPolicy)
	assert.NotNil(t, withPol.Policy)
	// Original unchanged.
	assert.Nil(t, base.Policy)
}

// --- WorkflowTrigger builder ---

func TestOnCreate_Build(t *testing.T) {
	tr := sdk.OnCreate("MyWorkflow", "mod.item").Build()
	assert.Equal(t, def.EventOnCreate, tr.On)
	assert.Equal(t, "MyWorkflow", tr.WorkflowFn)
	assert.Equal(t, "mod.item", tr.TaskQueue)
}

func TestOnSubmit_WithInputFn(t *testing.T) {
	called := false
	tr := sdk.OnSubmit("SubmitWF", "q").
		InputFn(func(rec *def.EntityRecord, tc def.TriggerContext) (any, error) {
			called = true
			return nil, nil
		}).Build()

	assert.Equal(t, def.EventOnSubmit, tr.On)
	require.NotNil(t, tr.InputBuilder)
	_, _ = tr.InputBuilder(nil, def.TriggerContext{})
	assert.True(t, called)
}

func TestAllTriggerEvents(t *testing.T) {
	cases := []struct {
		fn    func(string, string) *sdk.TriggerBuilder
		event def.EventType
	}{
		{sdk.OnCreate, def.EventOnCreate},
		{sdk.OnUpdate, def.EventOnUpdate},
		{sdk.OnDelete, def.EventOnDelete},
		{sdk.OnSubmit, def.EventOnSubmit},
		{sdk.OnApprove, def.EventOnApprove},
		{sdk.OnCancel, def.EventOnCancel},
	}
	for _, tc := range cases {
		t.Run(string(tc.event), func(t *testing.T) {
			tr := tc.fn("WF", "q").Build()
			assert.Equal(t, tc.event, tr.On)
		})
	}
}
