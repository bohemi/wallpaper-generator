CREATE TABLE IF NOT EXISTS wallpapers (
    id SERIAL PRIMARY KEY,
    image_url TEXT NOT NULL UNIQUE,
    source VARCHAR(100) DEFAULT 'pinterest',
    caption TEXT,
    is_served BOOLEAN DEFAULT FALSE,
    served_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
-- create index pointer on the is_served column of wallpapers table. it will make pointers on is_served column so when the user from the phone
-- makes a wallpaper request which are not true on is_served column from above table (see above table) then the pointers on is_served column
-- will be faster to get requests since the pointers will give results faster with the pointers pointing to those false state of is_served
CREATE INDEX IF NOT EXISTS idx_wallpapers_unserved 
ON wallpapers (is_served) 
WHERE is_served = FALSE;