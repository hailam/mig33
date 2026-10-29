UPDATE sqlite_sequence SET seq = MAX(seq, ?) WHERE name = 'messages';
