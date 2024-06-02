create table if not exists users (
    id uuid not null primary key,
    name text not null,
    username text not null unique,
    password text not null
)