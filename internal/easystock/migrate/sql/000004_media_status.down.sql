ALTER TABLE products DROP CONSTRAINT IF EXISTS products_image_media_fk;
ALTER TABLE products DROP COLUMN IF EXISTS image_media_id;
ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS media_objects_file_name_chk;
ALTER TABLE media_objects DROP CONSTRAINT IF EXISTS media_objects_status_chk;
ALTER TABLE media_objects DROP COLUMN IF EXISTS file_name;
ALTER TABLE media_objects DROP COLUMN IF EXISTS status;
