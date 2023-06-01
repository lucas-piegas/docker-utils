#!/bin/bash

# Replace "database_name" and "username" with your database name and MySQL username respectively

CREATE_TRANSACTION_TABLE="CREATE TABLE transaction (
    id               bigint       AUTO_INCREMENT PRIMARY KEY,
    amount decimal(19,2),
    created_at    datetime
);"

mysql -h 127.0.0.1 -P 3306 -u root -p1234 -e "CREATE DATABASE bdd_poc;"
echo "$CREATE_TRANSACTION_TABLE" > table_script.sql
mysql -h 127.0.0.1 -P 3306 -u root -p1234 bdd_poc < table_script.sql