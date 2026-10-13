-- Upload state for images and receipts. The object itself lives in S3-compatible storage.
-- pending: the row reserves the key and the declared size, the bytes are not confirmed yet.
-- ready: the object was found and its size is within the declared limit.

ALTER TABLE media_objects
  ADD COLUMN status text NOT NULL DEFAULT 'pending',
  ADD COLUMN file_name text;

ALTER TABLE media_objects
  ADD CONSTRAINT media_objects_status_chk CHECK (status IN ('pending', 'ready')),
  ADD CONSTRAINT media_objects_file_name_chk CHECK (file_name IS NULL OR char_length(file_name) <= 180);

ALTER TABLE products
  ADD COLUMN image_media_id uuid;

ALTER TABLE products
  ADD CONSTRAINT products_image_media_fk
  FOREIGN KEY (company_id, image_media_id) REFERENCES media_objects (company_id, id);
