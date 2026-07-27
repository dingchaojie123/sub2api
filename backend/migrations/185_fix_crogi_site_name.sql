-- Fix the temporary misspelled brand name saved in runtime settings.
UPDATE settings
SET value = 'Corgi',
    updated_at = NOW()
WHERE key = 'site_name'
  AND value = 'Crogi';
