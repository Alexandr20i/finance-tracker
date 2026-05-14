#!/bin/sh
echo "Applying budget migrations..."
psql "$DATABASE_URL" -f /migrations/001_init.sql
echo "Done!"