-- name: CreateShare :one
INSERT INTO shares (
    session_id,
    party_id,
    public_key,
    address,
    share_data,
    threshold,
    total_parties,
    party_index,
    curve
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetShareBySessionAndParty :one
SELECT * FROM shares
WHERE session_id = $1 AND party_id = $2
LIMIT 1;

-- name: GetShareByID :one
SELECT * FROM shares
WHERE id = $1
LIMIT 1;

-- name: GetSharesByPartyID :many
SELECT * FROM shares
WHERE party_id = $1
ORDER BY created_at DESC;

-- name: GetSharesByAddress :many
SELECT * FROM shares
WHERE address = $1
ORDER BY created_at DESC;

-- name: GetSharesByPublicKey :many
SELECT * FROM shares
WHERE public_key = $1
ORDER BY created_at DESC;

-- name: ListShares :many
SELECT * FROM shares
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateShareData :one
UPDATE shares
SET share_data = $1
WHERE id = $2
RETURNING *;

-- name: DeleteShare :exec
DELETE FROM shares
WHERE id = $1;

-- name: DeleteShareBySessionAndParty :exec
DELETE FROM shares
WHERE session_id = $1 AND party_id = $2;

-- name: CountSharesByPartyID :one
SELECT COUNT(*) FROM shares
WHERE party_id = $1;

-- name: ShareExists :one
SELECT EXISTS(
    SELECT 1 FROM shares
    WHERE session_id = $1 AND party_id = $2
);
