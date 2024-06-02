create table if not exists opaque_jwt_token (
    opaque text not null unique,
    jwt text not null unique,
    primary key(opaque, jwt)
)