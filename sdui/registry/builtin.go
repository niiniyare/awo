package registry

import "awo.so/awo/sdui/widget"

func init() {
	builtins := []WidgetDef{
		// ── Structural ────────────────────────────────────────────────────────
		{Kind: widget.NodePage, DisplayName: "Page", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeForm, DisplayName: "Form", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeList, DisplayName: "List", IsContainer: true, RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeSection, DisplayName: "Section", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTabPane, DisplayName: "Tab Pane", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTabs, DisplayName: "Tabs", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTable, DisplayName: "Table", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeGrid, DisplayName: "Grid", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeDialog, DisplayName: "Dialog", IsContainer: true, FrameworkVersion: "1.0"},

		// ── Input fields ──────────────────────────────────────────────────────
		{Kind: widget.NodeField, DisplayName: "Field (legacy)", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeText, DisplayName: "Text", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTextArea, DisplayName: "Text Area", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeRichText, DisplayName: "Rich Text", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeNumber, DisplayName: "Number", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeMoney, DisplayName: "Money", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeSelect, DisplayName: "Select", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeMultiSelect, DisplayName: "Multi Select", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeLookup, DisplayName: "Lookup", IsInputField: true, RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTreeSelect, DisplayName: "Tree Select", IsInputField: true, RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeDate, DisplayName: "Date", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeDateTime, DisplayName: "Date Time", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeDuration, DisplayName: "Duration", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeSwitch, DisplayName: "Switch", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeEditor, DisplayName: "JSON Editor", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeColor, DisplayName: "Color", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeSignature, DisplayName: "Signature", IsInputField: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeFileUpload, DisplayName: "File Upload", IsInputField: true, FrameworkVersion: "1.0"},

		// ── Display ───────────────────────────────────────────────────────────
		{Kind: widget.NodeStaticText, DisplayName: "Static Text", FrameworkVersion: "1.0"},
		{Kind: widget.NodeBadge, DisplayName: "Badge", FrameworkVersion: "1.0"},
		{Kind: widget.NodeSummaryCard, DisplayName: "Summary Card", IsContainer: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeWorkflowPanel, DisplayName: "Workflow Panel", FrameworkVersion: "1.0"},
		{Kind: widget.NodeAttachments, DisplayName: "Attachments", FrameworkVersion: "1.0"},
		{Kind: widget.NodeActivity, DisplayName: "Activity Feed", FrameworkVersion: "1.0"},
		{Kind: widget.NodeRelatedList, DisplayName: "Related List", IsContainer: true, RequiresDataSource: true, FrameworkVersion: "1.0"},

		// ── Dashboard ─────────────────────────────────────────────────────────
		{Kind: widget.NodeKPICard, DisplayName: "KPI Card", RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeChartPanel, DisplayName: "Chart Panel", RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeTablePanel, DisplayName: "Table Panel", RequiresDataSource: true, FrameworkVersion: "1.0"},
		{Kind: widget.NodeFilterBar, DisplayName: "Filter Bar", IsContainer: true, FrameworkVersion: "1.0"},

		// ── Interactive ───────────────────────────────────────────────────────
		{Kind: widget.NodeButton, DisplayName: "Button", FrameworkVersion: "1.0"},
	}

	for _, def := range builtins {
		Register(def) // panics on duplicate — caught at startup
	}
}
