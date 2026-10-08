INSERT INTO tables (id, name, max_players) VALUES
    ('apple',   'Apple',   2),
    ('butter',  'Butter',  2),
    ('charlie', 'Charlie', 2),
    ('delta',   'Delta',   2),
    ('echo',    'Echo',    2),
    ('foxtrot', 'Foxtrot', 2)
ON CONFLICT (id) DO NOTHING;