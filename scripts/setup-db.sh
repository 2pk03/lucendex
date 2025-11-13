#!/bin/bash
set -e

# Lucendex Database Setup Script
# Creates database and runs all migrations

DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-lucendex}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}

echo "🗄️  Setting up Lucendex database..."
echo ""

# Check if PostgreSQL is running
if ! pg_isready -h $DB_HOST -p $DB_PORT &>/dev/null; then
    echo "❌ PostgreSQL is not running on $DB_HOST:$DB_PORT"
    echo ""
    echo "Start PostgreSQL with:"
    echo "  docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=$DB_PASSWORD postgres:15"
    echo ""
    exit 1
fi

# Create database if doesn't exist
echo "Creating database '$DB_NAME'..."
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
    psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE $DB_NAME"

# Run schema initialization
echo "Running schema initialization..."
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME < backend/db/schema.sql

# Run migrations in order
echo "Running migrations..."
for migration in backend/db/migrations/*.sql; do
    if [ -f "$migration" ]; then
        echo "  ✓ $(basename $migration)"
        psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME < "$migration"
    fi
done

echo ""
echo "✅ Database setup complete!"
echo ""
echo "Connection details:"
echo "  Host: $DB_HOST"
echo "  Port: $DB_PORT"
echo "  Database: $DB_NAME"
echo "  User: $DB_USER"
echo ""
echo "Created roles:"
echo "  - indexer_rw (read/write for indexer)"
echo "  - router_ro (read-only for router)"
echo "  - api_ro (read-only for API + metering write)"
echo ""
echo "Next steps:"
echo "  1. Set passwords for database roles"
echo "  2. Configure environment variables for services"
echo "  3. Start indexer and API server"
