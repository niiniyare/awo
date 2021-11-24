package main

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetAircraftsDataTable(ctx *context.Context) table.Table {

	aircraftsData := table.NewDefaultTable(table.DefaultConfigWithDriver("postgresql"))

	info := aircraftsData.GetInfo().HideFilterArea()

	info.AddField("Aircraft_code", "aircraft_code", db.Varchar)
	info.AddField("Model", "model", db.Varchar)
	info.AddField("Range", "range", db.Int4)
	info.AddField("Company_id", "company_id", db.Int8)
	info.AddField("Created_at", "created_at", db.Timestamptz)

	info.SetTable("public.aircrafts_data").SetTitle("AircraftsData").SetDescription("AircraftsData")

	formList := aircraftsData.GetForm()
	formList.AddField("Aircraft_code", "aircraft_code", db.Varchar, form.Text)
	formList.AddField("Model", "model", db.Varchar, form.Text)
	formList.AddField("Range", "range", db.Int4, form.Number)
	formList.AddField("Company_id", "company_id", db.Int8, form.Text)
	formList.AddField("Created_at", "created_at", db.Timestamptz, form.Datetime)

	formList.SetTable("public.aircrafts_data").SetTitle("AircraftsData").SetDescription("AircraftsData")

	return aircraftsData
}
