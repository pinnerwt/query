-- name: CreateMenuPhotoUpload :one
INSERT INTO menu_photo_uploads (restaurant_id, file_path, file_name)
VALUES (@restaurant_id, @file_path, @file_name)
RETURNING *;

-- name: ListMenuPhotoUploadsByRestaurant :many
SELECT * FROM menu_photo_uploads WHERE restaurant_id = $1 ORDER BY created_at;

-- name: UpdateMenuPhotoUploadStatus :one
UPDATE menu_photo_uploads SET ocr_status = @ocr_status, ocr_text = @ocr_text
WHERE id = @id
RETURNING *;

-- name: DeleteMenuPhotoUploadsByRestaurant :exec
DELETE FROM menu_photo_uploads WHERE restaurant_id = $1;
