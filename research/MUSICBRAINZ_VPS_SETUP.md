# MusicBrainz VPS Database Setup Guide

## Goal

Set up a VPS running PostgreSQL with the complete MusicBrainz database using Docker. This will serve as a centralized music metadata service that can be used by multiple applications.

## Why Separate VPS?

- Download the full MusicBrainz dataset once (~100GB)
- Share database across multiple applications
- Keep data synchronized with integrated replication
- Independent deployment and scaling

## VPS Requirements

### Minimum Specifications
- **Storage**: 100GB minimum
- **RAM**: 2GB minimum (4GB recommended for better performance)
- **CPU**: 2 cores
- **OS**: Linux (Ubuntu/Debian recommended)
- **Docker**: Latest stable version
- **Docker Compose**: Latest stable version

### Estimated Costs
- VPS hosting: ~$20-40/month
- Initial bandwidth: ~20-30GB download for database dump
- Ongoing bandwidth: Minimal (hourly replication packets are small)

## Implementation Approach

### Using MusicBrainz Docker (alt-db-only-mirror)

Use the official [musicbrainz-docker](https://github.com/metabrainz/musicbrainz-docker) project with the `alt-db-only-mirror` configuration for a database-only setup without the web interface.

**Why Docker approach:**
- **Official support**: Maintained by MetaBrainz team
- **Integrated replication**: Built-in scripts that just work
- **Proven reliability**: Used by official MusicBrainz mirrors
- **Future-proof**: Automatic updates for schema changes
- **Complete tooling**: Backup, maintenance scripts included
- **Minimal overhead**: Only 100GB storage, 2GB RAM needed

**What gets installed:**
- PostgreSQL 16 database with full MusicBrainz data
- Replication infrastructure
- Redis (for caching)
- No web server or API components

## Key Concepts: Host vs Container Commands

**IMPORTANT**: Understanding where commands run is critical for this setup.

### Host Commands (run on your VPS terminal)
- **`admin/configure`**: Initial repository configuration (rarely needed)
- **`docker compose up -d`**: Start/stop containers
- **`docker compose ps`**: Check container status
- **`docker compose logs`**: View logs

### Container Commands (run INSIDE containers via `docker compose run` or `exec`)
- **`createdb.sh -fetch`**: Download and import database dump
- **`recreatedb.sh -fetch`**: Drop and recreate database with fresh dump
- **`replication.sh`**: Trigger replication manually
- **Database operations**: All `psql` commands run inside the `db` container

### Service Names
The alt-db-only-mirror setup uses these service names:
- **`db`**: PostgreSQL database server
- **`musicbrainz`**: Helper container for database operations (import, replication)
- **`redis`**: Redis cache

Use these names with `docker compose exec <service>` or `docker compose run <service>`.

## Setup Steps

### Phase 1: VPS Provisioning & Docker Installation

1. **Provision VPS** with required specifications

2. **Create non-root user with sudo access (Security best practice)**
   ```bash
   # Run as root
   adduser jelle
   usermod -aG sudo jelle
   usermod -aG docker jelle
   
   # Optional: Copy SSH keys
   mkdir -p /home/jelle/.ssh
   cp /root/.ssh/authorized_keys /home/jelle/.ssh/
   chown -R jelle:jelle /home/jelle/.ssh
   chmod 700 /home/jelle/.ssh
   chmod 600 /home/jelle/.ssh/authorized_keys
   
   # Log out and SSH back in as: ssh jelle@your-vps-ip
   ```

3. **Install Docker and Docker Compose**
   ```bash
   # Update system
   sudo apt update && sudo apt upgrade -y
   
   # Install Docker
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   
   # Add your user to docker group
   sudo usermod -aG docker $USER
   
   # Install Docker Compose
   sudo apt install docker-compose-plugin
   
   # Verify installation
   docker --version
   docker compose version
   ```

4. **Configure firewall**
   ```bash
   # Allow SSH
   sudo ufw allow 22/tcp
   
   # Allow PostgreSQL from specific IPs only (do this later)
   # sudo ufw allow from YOUR_APP_SERVER_IP to any port 5432
   
   # Enable firewall
   sudo ufw enable
   ```

### Phase 2: Clone and Configure MusicBrainz Docker

1. **Clone the repository**
   ```bash
   cd ~
   git clone https://github.com/metabrainz/musicbrainz-docker
   cd musicbrainz-docker
   ```

2. **Use the alt-db-only-mirror configuration**

   The repository includes `docker-compose.alt.db-only-mirror.yml` which provides a minimal database-only setup. Docker Compose will automatically use this configuration.

   **Note**: The admin scripts in the `admin/` directory on the host are only for initial repository configuration. Database operations (import, replication) run inside containers using `docker compose run` or `docker compose exec`.

3. **Create and configure .env file (Optional)**
   ```bash
   cp default.env .env
   ```

   Edit `.env` if you need to customize settings:
   ```bash
   # Database configuration
   POSTGRES_VERSION=16

   # Optional: Adjust memory if needed (default is 2GB)
   # MUSICBRAINZ_POSTGRES_SHARED_BUFFERS=2GB

   # Optional: Customize download URL
   # MUSICBRAINZ_BASE_DOWNLOAD_URL=https://data.metabrainz.org/pub/musicbrainz
   ```

   **Note**: For a database-only mirror, you typically don't need a MetaBrainz access token. This is only required for live replication or API access.

### Phase 3: Initial Database Import

1. **Start the base services**
   ```bash
   sudo docker compose up -d
   ```

   This starts the PostgreSQL database (`db`), Redis, and the `musicbrainz` service containers.

2. **Download and import the database dump**

   **IMPORTANT**: This is the long step that can take 4-8 hours depending on your VPS specs and network speed.

   ```bash
   # This downloads the latest full dump (~20-30GB) and imports it
   sudo docker compose run --rm musicbrainz createdb.sh -fetch
   ```

   This command will:
   - Download the latest MusicBrainz data dump files
   - Extract them
   - Import into PostgreSQL
   - Create indexes and constraints

   The process runs in the foreground and you'll see progress output. You can monitor logs in a separate terminal:
   ```bash
   sudo docker compose logs -f musicbrainz
   ```

3. **Verify import**
   ```bash
   sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db \
     -c "SELECT COUNT(*) FROM musicbrainz.artist;"
   ```

   You should see a count of over 1 million artists if the import was successful.

### Phase 4: Set Up Replication (Optional but Recommended)

Replication keeps your database up-to-date with the latest changes from MusicBrainz. The database dump you imported is a snapshot in time; replication applies incremental updates.

**Note**: The alt-db-only-mirror setup includes a `musicbrainz` service with `command: load-crontab-only.sh` which automatically handles replication scheduling. Check if replication is already configured:

```bash
sudo docker compose exec musicbrainz crontab -l
```

If you see cron entries, replication is already set up and you can skip manual configuration.

If you need to manually trigger replication:

1. **Initial replication to catch up from dump to current state**
   ```bash
   sudo docker compose run --rm musicbrainz replication.sh
   ```

2. **Monitor replication status**
   ```bash
   sudo docker compose logs -f musicbrainz
   ```

3. **Alternative: Set up host-level cron for replication**

   If automatic replication isn't working, create a cron script:
   ```bash
   sudo vim /etc/cron.hourly/musicbrainz-replication
   ```

   Add:
   ```bash
   #!/bin/bash
   cd /home/jelle/musicbrainz-docker
   docker compose exec -T musicbrainz /usr/local/bin/replication.sh
   ```

   Make it executable:
   ```bash
   sudo chmod +x /etc/cron.hourly/musicbrainz-replication
   ```

### Phase 5: Security & Access Configuration

1. **Create read-only database user for applications**
   ```bash
   sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db
   ```

   Then in PostgreSQL:
   ```sql
   -- Create read-only user
   CREATE USER readonly WITH PASSWORD 'strong_readonly_password';

   -- Grant connection
   GRANT CONNECT ON DATABASE musicbrainz_db TO readonly;

   -- Grant usage on schema
   GRANT USAGE ON SCHEMA musicbrainz TO readonly;

   -- Grant SELECT on all tables
   GRANT SELECT ON ALL TABLES IN SCHEMA musicbrainz TO readonly;

   -- Grant SELECT on future tables
   ALTER DEFAULT PRIVILEGES IN SCHEMA musicbrainz
     GRANT SELECT ON TABLES TO readonly;

   \q
   ```

2. **Configure PostgreSQL for remote connections**

   First, check the current PostgreSQL configuration:
   ```bash
   sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db \
     -c "SHOW listen_addresses;"
   ```

   Edit PostgreSQL config in Docker:
   ```bash
   sudo docker compose exec db bash

   # Inside container, edit pg_hba.conf
   echo "host all readonly YOUR_APP_IP/32 md5" >> /var/lib/postgresql/data/pg_hba.conf

   exit
   ```

   Restart to apply:
   ```bash
   sudo docker compose restart db
   ```

3. **Expose PostgreSQL port** (if needed)

   By default, the alt-db-only-mirror configuration only exposes port 5432 internally to other containers. To allow external connections, you need to publish the port to the host.

   Edit `docker-compose.yml` or create a `docker-compose.override.yml`:
   ```yaml
   services:
     db:
       ports:
         - "5432:5432"
   ```

   Then restart:
   ```bash
   sudo docker compose up -d
   ```

4. **Configure firewall to allow application access**
   ```bash
   # Allow PostgreSQL from specific application server IPs
   sudo ufw allow from YOUR_APP_SERVER_IP to any port 5432

   # Or for development, allow from your local IP
   sudo ufw allow from YOUR_DEV_MACHINE_IP to any port 5432
   ```

5. **Test remote connection**

   From your development machine:
   ```bash
   psql -h your-vps-ip.com -U readonly -d musicbrainz_db -c "SELECT version();"
   ```

### Phase 6: Optional - Set Up SSL/TLS

For production, configure SSL certificates:

1. **Generate SSL certificates** (or use Let's Encrypt)
2. **Mount certificates in Docker Compose**
3. **Configure PostgreSQL to require SSL**
4. **Update firewall rules if needed**

## Maintenance

### Regular Tasks

**Check container status:**
```bash
sudo docker compose ps
```

**View logs:**
```bash
sudo docker compose logs -f
```

**Monitor disk space:**
```bash
df -h
sudo docker system df
```

**Check replication status:**
```bash
# Check if cron is set up
sudo docker compose exec musicbrainz crontab -l

# View recent logs
sudo docker compose logs --tail=100 musicbrainz
```

### When Schema Updates Occur

MusicBrainz announces schema changes on their [blog](https://blog.metabrainz.org/). When this happens:

1. **Pull latest Docker images:**
   ```bash
   cd ~/musicbrainz-docker
   git pull
   sudo docker compose pull
   ```

2. **Follow upgrade instructions** (usually in release announcement)

3. **Restart services:**
   ```bash
   sudo docker compose down
   sudo docker compose up -d
   ```

4. **Re-test your application queries**

5. **Update schema documentation** in your app repos

### Backups

**Backup the database:**
```bash
sudo docker compose exec -T db pg_dump -U musicbrainz musicbrainz_db | \
  gzip > musicbrainz-backup-$(date +%Y%m%d).sql.gz
```

**Automate backups with cron:**
```bash
# /etc/cron.daily/musicbrainz-backup
#!/bin/bash
BACKUP_DIR=/backups/musicbrainz
mkdir -p $BACKUP_DIR
cd /home/jelle/musicbrainz-docker
docker compose exec -T db pg_dump -U musicbrainz musicbrainz_db | \
  gzip > $BACKUP_DIR/musicbrainz-$(date +%Y%m%d).sql.gz

# Keep only last 7 days
find $BACKUP_DIR -name "musicbrainz-*.sql.gz" -mtime +7 -delete
```

### Updates

**Update Docker images:**
```bash
cd ~/musicbrainz-docker
git pull
sudo docker compose pull
sudo docker compose up -d
```

## Application Integration

### Connection from Your Apps

**Environment variables:**
```bash
MUSICBRAINZ_DB_HOST=your-vps-ip.com
MUSICBRAINZ_DB_PORT=5432
MUSICBRAINZ_DB_NAME=musicbrainz_db
MUSICBRAINZ_DB_USER=readonly
MUSICBRAINZ_DB_PASSWORD=strong_readonly_password
```

**Example Node.js connection:**
```typescript
import { Pool } from 'pg';

const pool = new Pool({
  host: process.env.MUSICBRAINZ_DB_HOST,
  port: parseInt(process.env.MUSICBRAINZ_DB_PORT || '5432'),
  database: process.env.MUSICBRAINZ_DB_NAME || 'musicbrainz_db',
  user: process.env.MUSICBRAINZ_DB_USER,
  password: process.env.MUSICBRAINZ_DB_PASSWORD,
  ssl: process.env.NODE_ENV === 'production' ? { rejectUnauthorized: true } : false,
  max: 20,
  idleTimeoutMillis: 30000,
});

export { pool };
```

### Schema Introspection for Development

1. **Dump schema for reference:**
   ```bash
   psql -h your-vps-ip.com -U readonly -d musicbrainz_db --schema-only > musicbrainz-schema.sql
   ```

2. **Commit schema to your app repo** for documentation and reference

3. **Explore interactively:**
   ```bash
   psql -h your-vps-ip.com -U readonly -d musicbrainz_db
   ```
   
   Useful commands:
   ```sql
   \dt musicbrainz.*           -- List all tables
   \d musicbrainz.artist       -- Describe artist table
   \d+ musicbrainz.release_group -- Detailed table info
   
   -- Sample queries
   SELECT * FROM musicbrainz.artist LIMIT 5;
   SELECT COUNT(*) FROM musicbrainz.release_group;
   ```

4. **Test queries** directly against VPS or create local mirror for faster iteration

### Schema Update Frequency

- **Schema changes**: Few times per year (announced on [MusicBrainz blog](https://blog.metabrainz.org/))
- **Data updates**: Continuous via hourly/daily replication (your queries keep working)
- **Action needed**: Only when schema changes are announced

## Troubleshooting

### Container won't start
```bash
# Check logs
sudo docker compose logs

# Restart services
sudo docker compose restart

# Full restart
sudo docker compose down
sudo docker compose up -d
```

### Database import failed
```bash
# Check logs for errors
sudo docker compose logs musicbrainz

# Try recreating the database
sudo docker compose run --rm musicbrainz recreatedb.sh -fetch
```

### Replication falling behind
```bash
# Check if cron is running
sudo docker compose exec musicbrainz crontab -l

# View replication logs
sudo docker compose logs --tail=200 musicbrainz

# Manually trigger replication
sudo docker compose run --rm musicbrainz replication.sh
```

### Out of disk space
```bash
# Check Docker disk usage
sudo docker system df

# Clean up old images and containers
sudo docker system prune -a

# Check database size
sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db \
  -c "SELECT pg_size_pretty(pg_database_size('musicbrainz_db'));"
```

### Can't connect remotely
```bash
# Check if port is published
sudo docker compose ps
# Look for "0.0.0.0:5432->5432/tcp" in the db service

# Check PostgreSQL is listening
sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db \
  -c "SHOW listen_addresses;"

# Check firewall
sudo ufw status

# Check pg_hba.conf
sudo docker compose exec db cat /var/lib/postgresql/data/pg_hba.conf

# Test connection from VPS itself
psql -h localhost -U readonly -d musicbrainz_db -c "SELECT 1;"
```

## Resources & Documentation

### Official MusicBrainz Resources
- **MusicBrainz Docker Repository**: https://github.com/metabrainz/musicbrainz-docker
- **MusicBrainz Docker README**: https://github.com/metabrainz/musicbrainz-docker/blob/master/README.md
- **Datasets & Downloads**: https://metabrainz.org/datasets/postgres-dumps#musicbrainz
- **Database Documentation**: https://musicbrainz.org/doc/MusicBrainz_Database/Download
- **Replication API**: https://metabrainz.org/api/
- **MusicBrainz Blog**: https://blog.metabrainz.org/ (for schema update announcements)

### MusicBrainz Server
- **Server Repository**: https://github.com/metabrainz/musicbrainz-server

### Data Licensing
- **Core data**: CC0 (Public Domain)
- **Supplementary data**: CC BY-NC-SA 3.0
- **Commercial use**: Allowed, financial support encouraged

## Next Steps

1. [ ] Choose and provision VPS provider
2. [ ] Install Docker and Docker Compose
3. [ ] Clone musicbrainz-docker repository
4. [ ] Get MetaBrainz access token
5. [ ] Configure alt-db-only-mirror setup
6. [ ] Download and import initial database dump
7. [ ] Set up automated replication
8. [ ] Configure security and read-only user
9. [ ] Test remote connection from development machine
10. [ ] Dump schema for application development reference

## Questions to Resolve

- [ ] Which VPS provider? (DigitalOcean, Linode, Hetzner, etc.)
- [ ] Replication frequency: hourly or daily?
- [ ] Backup strategy: where to store, how long to retain?
- [ ] Monitoring solution: built-in or third-party?
- [ ] SSL/TLS setup: required for production?
- [ ] Multiple application servers: how many IPs to whitelist?

## Actual Setup Experience (Lessons Learned)

### What We Planned vs What Actually Worked

#### Phase 2: Configuration
- **Planned**: Run `sudo admin/configure with alt-db-only-mirror`
- **Actual**: Not needed! The repository automatically uses `docker-compose.alt.db-only-mirror.yml` when you start with `docker compose up -d`. The admin scripts on the host are only for initial repository configuration, not database operations.

#### Phase 3: Database Import
- **Planned**: `sudo admin/download-import-dump`
- **Actual**: `sudo docker compose run --rm musicbrainz createdb.sh -fetch`
- **Why**: Database operations run INSIDE containers, not from host admin scripts. The correct command runs the `createdb.sh` script inside the `musicbrainz` container.

#### Service Names
- **Planned**: Service named `musicbrainz-db`
- **Actual**: Service is named `db`
- **Impact**: All commands use `db` not `musicbrainz-db` (e.g., `docker compose exec db psql ...`)

#### MetaBrainz Token
- **Planned**: Required for setup
- **Actual**: Not needed for basic database-only mirror! Only required if you want live replication or API access.

#### Port Exposure
- **Not in original plan**: The alt-db-only-mirror setup only exposes port 5432 to other containers, not to the host. To allow remote connections, you must explicitly publish the port in a `docker-compose.override.yml` file.

### Commands Reference (Corrected)

```bash
# Start services
sudo docker compose up -d

# Download and import database (4-8 hours)
sudo docker compose run --rm musicbrainz createdb.sh -fetch

# Verify import
sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db -c "SELECT COUNT(*) FROM musicbrainz.artist;"

# Check running containers
sudo docker compose ps

# View logs
sudo docker compose logs -f musicbrainz

# Access database directly
sudo docker compose exec db psql -U musicbrainz -d musicbrainz_db
```

### Setup Time Estimates
- **Phase 1 (VPS setup)**: 30 minutes
- **Phase 2 (Clone & config)**: 10 minutes
- **Phase 3 (Database import)**: 4-8 hours (mostly automated, just waiting)
- **Phase 4 (Replication)**: Auto-configured via `load-crontab-only.sh`
- **Phase 5 (Security)**: 20 minutes
- **Total active time**: ~1 hour of work + 4-8 hours waiting for download/import
