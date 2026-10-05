-- SPDX-License-Identifier: Apache-2.0

CREATE SCHEMA auth;

CREATE TABLE auth.users (
    id uuid PRIMARY KEY,
    name text NOT NULL
);
