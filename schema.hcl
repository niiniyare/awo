-- Add new schema named "public"
CREATE SCHEMA IF NOT EXISTS "public";
-- Create "cabin_class" table
CREATE TABLE "public"."cabin_class" ("value" text NOT NULL, "description" text NOT NULL, PRIMARY KEY ("value"));
-- Create "schema_migrations" table
CREATE TABLE "public"."schema_migrations" ("version" bigint NOT NULL, "dirty" boolean NOT NULL, PRIMARY KEY ("version"));
-- Create "seats" table
CREATE TABLE "public"."seats" ("aircraft_id" bigserial NOT NULL, "seat_no" character varying(4) NOT NULL, "fare_conditions" character varying(10) NOT NULL, CONSTRAINT "seats_fare_conditions_check" CHECK ((fare_conditions)::text = ANY (ARRAY[('Economy'::character varying)::text, ('Comfort'::character varying)::text, ('Business'::character varying)::text])));
-- Set comment to column: "aircraft_id" on table: "seats"
COMMENT ON COLUMN "public"."seats" ."aircraft_id" IS 'Aircraft code, IATA';
-- Set comment to column: "seat_no" on table: "seats"
COMMENT ON COLUMN "public"."seats" ."seat_no" IS 'Seat number';
-- Set comment to column: "fare_conditions" on table: "seats"
COMMENT ON COLUMN "public"."seats" ."fare_conditions" IS 'Travel class';
-- Create "airlines" table
CREATE TABLE "public"."airlines" ("id" bigserial NOT NULL, "company_name" character varying(50) NOT NULL, "iata_code" character varying(2) NOT NULL, "registared_country" character varying(2) NOT NULL, "created_at" timestamptz NOT NULL DEFAULT now(), "updated_at" timestamptz NOT NULL DEFAULT '0001-01-01 02:27:16+02:27:16', PRIMARY KEY ("id"));
-- Create "flights" table
CREATE TABLE "public"."flights" ("flight_id" bigserial NOT NULL, "flight_no" character varying(6) NOT NULL, "company_id" bigserial NOT NULL, "scheduled_departure" timestamptz NOT NULL, "scheduled_arrival" timestamptz NOT NULL, "departure_airport" character varying(3) NOT NULL, "arrival_airport" character varying(3) NOT NULL, "status" character varying(20) NOT NULL, "aircraft_id" bigserial NOT NULL, "actual_departure" timestamptz NULL, "actual_arrival" timestamptz NULL, PRIMARY KEY ("flight_id"), CONSTRAINT "flights_company_id_fkey" FOREIGN KEY ("company_id") REFERENCES "public"."airlines" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "flights_check" CHECK (scheduled_arrival > scheduled_departure), CONSTRAINT "flights_check1" CHECK ((actual_arrival IS NULL) OR ((actual_departure IS NOT NULL) AND (actual_arrival IS NOT NULL) AND (actual_arrival > actual_departure))), CONSTRAINT "flights_status_check" CHECK ((status)::text = ANY (ARRAY[('On Time'::character varying)::text, ('Delayed'::character varying)::text, ('Departed'::character varying)::text, ('Arrived'::character varying)::text, ('Scheduled'::character varying)::text, ('Cancelled'::character varying)::text])));
-- Create index "flights_check_airlines_key" to table: "flights"
CREATE UNIQUE INDEX "flights_check_airlines_key" ON "public"."flights" ("flight_no", "company_id");
-- Create index "flights_flight_no_scheduled_departure_key" to table: "flights"
CREATE UNIQUE INDEX "flights_flight_no_scheduled_departure_key" ON "public"."flights" ("flight_no", "scheduled_departure");
-- Create "booked_seat" table
CREATE TABLE "public"."booked_seat" ("id" bigserial NOT NULL, "flight_id" integer NOT NULL, "cabin_class" text NOT NULL, "seat_row" integer NOT NULL, "seat_column" text NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "booked_seat_cabin_class_fkey" FOREIGN KEY ("cabin_class") REFERENCES "public"."cabin_class" ("value") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "booked_seat_flight_id_fkey" FOREIGN KEY ("flight_id") REFERENCES "public"."flights" ("flight_id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "booked_seat_seat_column_check" CHECK (seat_column ~ '\A[A-Z]\Z'::text), CONSTRAINT "booked_seat_seat_row_check" CHECK (seat_row > 0));
-- Create index "booked_seat_flight_id_seat_row_seat_column_key" to table: "booked_seat"
CREATE UNIQUE INDEX "booked_seat_flight_id_seat_row_seat_column_key" ON "public"."booked_seat" ("flight_id", "seat_row", "seat_column");
-- Create "aircrafts" table
CREATE TABLE "public"."aircrafts" ("id" bigserial NOT NULL, "iata_code" character varying(3) NOT NULL, "icao_code" character varying(4) NOT NULL, "model" character varying(100) NOT NULL, "range" integer NOT NULL DEFAULT 1, "company_id" bigserial NOT NULL, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "aircrafts_range_check" CHECK (range > 0));
-- Set comment to table: "aircrafts"
COMMENT ON TABLE "public"."aircrafts" IS 'Aircrafts (internal data)';
-- Set comment to column: "iata_code" on table: "aircrafts"
COMMENT ON COLUMN "public"."aircrafts" ."iata_code" IS 'Aircraft code, IATA';
-- Set comment to column: "icao_code" on table: "aircrafts"
COMMENT ON COLUMN "public"."aircrafts" ."icao_code" IS 'Aircraft code, ICAO';
-- Set comment to column: "model" on table: "aircrafts"
COMMENT ON COLUMN "public"."aircrafts" ."model" IS 'Aircraft model';
-- Set comment to column: "range" on table: "aircrafts"
COMMENT ON COLUMN "public"."aircrafts" ."range" IS 'Maximal flying distance, km';
-- Create "seat_map" table
CREATE TABLE "public"."seat_map" ("id" bigserial NOT NULL, "aircraft_id" integer NOT NULL, "cabin_class" text NOT NULL, "start_row" integer NOT NULL, "end_row" integer NOT NULL, "column_layout" text NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "seat_map_aircraft_id_fkey" FOREIGN KEY ("aircraft_id") REFERENCES "public"."aircrafts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "seat_map_cabin_class_fkey" FOREIGN KEY ("cabin_class") REFERENCES "public"."cabin_class" ("value") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "seat_map_check" CHECK (start_row <= end_row), CONSTRAINT "seat_map_column_layout_check" CHECK (column_layout ~ '\A[A-Z#]+(?:-[A-Z#]+)*\Z'::text), CONSTRAINT "seat_map_end_row_check" CHECK (end_row > 0), CONSTRAINT "seat_map_start_row_check" CHECK (start_row > 0));
-- Create index "seat_map_aircraft_id_start_row_end_row_key" to table: "seat_map"
CREATE UNIQUE INDEX "seat_map_aircraft_id_start_row_end_row_key" ON "public"."seat_map" ("aircraft_id", "start_row", "end_row");
