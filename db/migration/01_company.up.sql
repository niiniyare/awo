
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;



--CREATE DATABASE flightBookings;


--\connect flightBookings



--CREATE SCHEMA bookings;



-- COMMENT  ON SCHEMA bookings IS 'Airlines flightBookings database schema';



--CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;



--  COMMENT  ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


--SET search_path = bookings, pg_catalog;


CREATE TABLE IF NOT EXISTS airlines (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    company_name VARCHAR(50) NOT NULL,
    iata_code VARCHAR(5) NOT NULL,
    main_airport VARCHAR(3) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT (now())
 -- account numeric NOT NULL,
 --   CONSTRAINT airlines_pk PRIMARY KEY (id)
  );

CREATE SEQUENCE IF NOT EXISTS airlines_id_seq
    START WITH 4001
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



ALTER SEQUENCE airlines_id_seq OWNED BY airlines.id;

