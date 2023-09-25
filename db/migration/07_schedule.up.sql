-- PostgreSQL schema for flight data

CREATE TABLE flights (
  -- Airline code
  airline_code VARCHAR(2) NOT NULL,
  -- Flight number
  flight_number INTEGER NOT NULL,
  -- Date range of the flight
  date_range DATERANGE NOT NULL,
  -- Day of week of the flight
  dow INTEGER NOT NULL,
  -- Legs of the flight
  legs JSONB NOT NULL,
  -- Segments of the flight
  segments JSONB NOT NULL,
  -- Primary key
  PRIMARY KEY (airline_code, flight_number),
  -- GUID
  guid UUID NOT NULL DEFAULT uuid_generate_v4(),
  -- Created at
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Updated at
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE legs (
  -- Boarding point
  board_point VARCHAR(3) NOT NULL,
  -- Off point
  off_point VARCHAR(3) NOT NULL,
  -- Boarding time
  board_time TIME NOT NULL,
  -- Arrival date offset
  arrival_date_offset INTEGER NOT NULL,
  -- Arrival time
  arrival_time TIME NOT NULL,
  -- Elapsed time
  elapsed_time TIME NOT NULL,
  -- Leg cabins
  leg_cabins JSONB NOT NULL,
  -- Foreign key to flights table
  FOREIGN KEY (airline_code, flight_number) REFERENCES flights (airline_code, flight_number),
  -- Index on boarding point and off point
  INDEX (board_point, off_point),
  -- GUID
  guid UUID NOT NULL DEFAULT uuid_generate_v4(),
  -- Created at
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Updated at
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE leg_cabins (
  -- Cabin code
  cabin_code VARCHAR(1) NOT NULL,
  -- Cabin capacity
  capacity INTEGER NOT NULL,
  -- Primary key
  PRIMARY KEY (cabin_code),
  -- GUID
  guid UUID NOT NULL DEFAULT uuid_generate_v4(),
  -- Created at
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Updated at
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE segments (
  -- Whether the segment is specific
  specific BOOLEAN NOT NULL,
  -- Fare family
  fare_family VARCHAR(255) NOT NULL,
  -- Disutility curve
  disutility_curve VARCHAR(255) NOT NULL,
  -- Class
  class VARCHAR(1) NOT NULL,
  -- Foreign key to legs table
  FOREIGN KEY (airline_code, flight_number, leg_number) REFERENCES legs (airline_code, flight_number, leg_number),
  -- GUID
  guid UUID NOT NULL DEFAULT uuid_generate_v4(),
  -- Created at
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Updated at
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

