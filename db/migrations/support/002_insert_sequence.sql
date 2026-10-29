INSERT INTO sqlite_sequence(name, seq)
SELECT 'messages', ? WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'messages');
