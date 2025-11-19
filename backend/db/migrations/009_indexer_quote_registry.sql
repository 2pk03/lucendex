-- Grant indexer role access to quote registry cleanup

GRANT SELECT, DELETE ON quote_registry TO indexer_rw;
