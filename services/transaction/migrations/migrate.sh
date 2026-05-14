#!/bin/sh
echo "Applying transaction migrations..."
psql "$DATABASE_URL" -f /migrations/001_init.sql
echo "Done!"