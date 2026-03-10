-- name: CreateOwner :one
INSERT INTO owners (email, password_hash, name, phone)
VALUES (@email, @password_hash, @name, @phone)
RETURNING *;

-- name: GetOwnerByEmail :one
SELECT * FROM owners WHERE email = $1;

-- name: GetOwnerByID :one
SELECT * FROM owners WHERE id = $1;
