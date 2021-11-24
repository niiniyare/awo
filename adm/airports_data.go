package main

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetAirportsDataTable(ctx *context.Context) table.Table {

	airportsData := table.NewDefaultTable(table.DefaultConfigWithDriver("postgresql"))

	info := airportsData.GetInfo().HideFilterArea()

	info.AddField("Airport_code", "airport_code", db.Varchar)
	info.AddField("Airport_name", "airport_name", db.Text)
	info.AddField("Country_code", "country_code", db.Varchar)
	info.AddField("City", "city", db.Text)
	info.AddField("Coordinates", "coordinates", db.Point)
	info.AddField("Timezone", "timezone", db.Text)
	info.AddField("Created_at", "created_at", db.Timestamptz)

	info.SetTable("public.airports_data").SetTitle("AirportsData").SetDescription("AirportsData")

	formList := airportsData.GetForm()
	formList.AddField("Airport_code", "airport_code", db.Varchar, form.Text)
	formList.AddField("Airport_name", "airport_name", db.Text, form.RichText)
	formList.AddField("Country_code", "country_code", db.Varchar, form.Text)
	formList.AddField("City", "city", db.Text, form.RichText)
	formList.AddField("Coordinates", "coordinates", db.Point, form.Text)
	formList.AddField("Timezone", "timezone", db.Text, form.RichText)
	formList.AddField("Created_at", "created_at", db.Timestamptz, form.Datetime)

	formList.SetTable("public.airports_data").SetTitle("AirportsData").SetDescription("AirportsData")

	return airportsData
}
