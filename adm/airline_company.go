package main

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetAirlineCompanyTable(ctx *context.Context) table.Table {

	airlineCompany := table.NewDefaultTable(table.DefaultConfigWithDriver("postgresql"))

	info := airlineCompany.GetInfo().HideFilterArea()

	info.AddField("Company_id", "company_id", db.Int8)
	info.AddField("Company_name", "company_name", db.Varchar)
	info.AddField("Iata_code", "iata_code", db.Varchar)
	info.AddField("Main_airport", "main_airport", db.Varchar)
	info.AddField("Created_at", "created_at", db.Timestamptz)

	info.SetTable("public.airline_company").SetTitle("AirlineCompany").SetDescription("AirlineCompany")

	formList := airlineCompany.GetForm()
	formList.AddField("Company_id", "company_id", db.Int8, form.Text)
	formList.AddField("Company_name", "company_name", db.Varchar, form.Text)
	formList.AddField("Iata_code", "iata_code", db.Varchar, form.Text)
	formList.AddField("Main_airport", "main_airport", db.Varchar, form.Text)
	formList.AddField("Created_at", "created_at", db.Timestamptz, form.Datetime)

	formList.SetTable("public.airline_company").SetTitle("AirlineCompany").SetDescription("AirlineCompany")

	return airlineCompany
}
