-- Whether this link offers downloads, chosen by whoever shared it. Existing links
-- keep downloads on. JFSHARE_ALLOW_DOWNLOADS=false still switches them off for
-- every link.
ALTER TABLE shares ADD COLUMN allow_download BOOLEAN NOT NULL DEFAULT TRUE;
