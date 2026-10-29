SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'messages'), 0);
