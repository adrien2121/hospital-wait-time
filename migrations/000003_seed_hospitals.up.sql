INSERT INTO hospitals (id, name, address, facility_type, source_url, active) VALUES
    ('cheo',        'Children''s Hospital of Eastern Ontario', '401 Smyth Rd, Ottawa, ON K1H 8L1',  'er', 'https://www.cheo.on.ca',             TRUE),
    ('montfort',    'Hôpital Montfort',                        '713 Montreal Rd, Ottawa, ON K1K 0T2', 'er', 'https://hopitalmontfort.com',        TRUE),
    ('toh-civic',   'The Ottawa Hospital – Civic Campus',      '1053 Carling Ave, Ottawa, ON K1Y 4E9','er', 'https://www.ottawahospital.on.ca',   FALSE),
    ('toh-general', 'The Ottawa Hospital – General Campus',    '501 Smyth Rd, Ottawa, ON K1H 8L6',   'er', 'https://www.ottawahospital.on.ca',   FALSE),
    ('qch',         'Queensway Carleton Hospital',             '3045 Baseline Rd, Ottawa, ON K2H 8P4','er', 'https://www.qch.on.ca',             FALSE)
ON CONFLICT (id) DO NOTHING;
