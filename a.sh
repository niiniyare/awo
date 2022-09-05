#!/bin/sh

THE_USER=admin
THE_DB=flight
THE_TABLE=airports
PSQL=/data/data/com.termux/files/usr/bin/psql
THE_DIR=util/sample_data
THE_FILE=airports.csv
${PSQL} -U ${THE_USER} ${THE_DB} <<OMG
\COPY ${THE_TABLE} FROM '${THE_DIR}/${THE_FILE}' delimiter '|' csv;
OMG
