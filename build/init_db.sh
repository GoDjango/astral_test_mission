#!/bin/bash

set -e

psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  CREATE TABLE public.users (
      id SERIAL PRIMARY KEY,
      login VARCHAR(255) NOT NULL,
      password VARCHAR(255) NOT NULL,
      UNIQUE(login)
  );

  CREATE TABLE public.docs (
      id SERIAL PRIMARY KEY,
      filename VARCHAR(255) NOT NULL,
      public boolean NOT NULL,
      mime VARCHAR(255) NOT NULL,
      owner_id integer NOT NULL,
      created timestamp NOT NULL,
      CONSTRAINT fk_user FOREIGN KEY(owner_id) REFERENCES users(id) ON DELETE CASCADE,
      UNIQUE(filename)
  );

  CREATE TABLE public.users_docs_grant (
      doc_id integer NOT NULL,
      user_id integer NOT NULL,
      CONSTRAINT fk_doc FOREIGN KEY(doc_id) REFERENCES docs(id) ON DELETE CASCADE,
      CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
      UNIQUE(doc_id, user_id)
  );

  CREATE TABLE public.tokens (
      user_id integer NOT NULL,
      token VARCHAR(255) NOT NULL,
      CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
      UNIQUE(token)
  );
EOSQL
