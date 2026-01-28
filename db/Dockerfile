# PostgreSQL Database for GPU Telemetry
# Based on official PostgreSQL 15 Alpine image

FROM postgres:15-alpine

# Labels
LABEL maintainer="GPU Telemetry Pipeline"
LABEL description="PostgreSQL database for GPU telemetry storage"

# Environment variables (can be overridden)
ENV POSTGRES_DB=telemetry
ENV POSTGRES_USER=postgres
ENV POSTGRES_PASSWORD=postgres

# Copy initialization scripts
# These run in alphabetical order when container starts for first time
COPY init/init.sql /docker-entrypoint-initdb.d/00-init.sql
COPY migrations/ /docker-entrypoint-initdb.d/migrations/

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD pg_isready -U $POSTGRES_USER -d $POSTGRES_DB || exit 1

# Expose PostgreSQL port
EXPOSE 5432

# Data volume
VOLUME ["/var/lib/postgresql/data"]
