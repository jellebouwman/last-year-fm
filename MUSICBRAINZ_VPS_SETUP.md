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

2. **Run configuration for database-only mirror**
   ```bash
   sudo admin/configure with alt-db-only-mirror
   ```

3. **Create and configure .env file**
   ```bash
   cp default.env .env
   ```
   
   Edit `.env` and configure:
   ```bash
   # Required: Your MetaBrainz access token
   METABRAINZ_ACCESS_TOKEN=your_token_here
   
   # Database configuration
   POSTGRES_VERSION=16
   MUSICBRAINZ_POSTGRES_PORT=5432
   
   # Optional: Adjust memory if needed
   MUSICBRAINZ_POSTGRES_SHARED_BUFFERS=2GB
   ```

4. **Get MetaBrainz API token**
   - Create account at https://metabrainz.org
   - Go to your profile page
   - Copy your access token
   - Add it to `.env` file as `METABRAINZ_ACCESS_TOKEN`

### Phase 3: Initial Database Import

1. **Start the services**
   ```bash
   sudo docker compose up -d
   ```

2. **Download and import the database dump**
   ```bash
   # This downloads the latest dump and imports it
   # Takes several hours depending on your connection and CPU
   sudo admin/download-import-dump
   ```
   
   Monitor progress:
   ```bash
   sudo docker compose logs -f musicbrainz
   ```

3. **Verify import**
   ```bash
   sudo docker compose exec musicbrainz-db psql -U musicbrainz -d musicbrainz_db \
     -c "SELECT COUNT(*) FROM artist;"
   ```

### Phase 4: Set Up Replication

1. **Initial replication to catch up**
   ```bash
   # Catch up from dump to current state
   sudo admin/replication-up
   ```

2. **Set up automated replication cron job**
   
   Create a cron script:
   ```bash
   sudo nano /etc/cron.hourly/musicbrainz-replication
   ```
   
   Add:
   ```bash
   #!/bin/bash
   cd /home/YOUR_USER/musicbrainz-docker
   docker compose exec -T musicbrainz admin/cron/hourly.sh
   ```
   
   Make it executable:
   ```bash
   sudo chmod +x /etc/cron.hourly/musicbrainz-replication
   ```
   
   Or for daily replication, use `/etc/cron.daily/` instead.

3. **Verify replication is working**
   ```bash
   sudo docker compose exec musicbrainz admin/replication-status
   ```

### Phase 5: Security & Access Configuration

1. **Create read-only database user for applications**
   ```bash
   sudo docker compose exec musicbrainz-db psql -U musicbrainz -d musicbrainz_db
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
   
   Edit PostgreSQL config in Docker:
   ```bash
   sudo docker compose exec musicbrainz-db bash
   
   # Inside container, edit pg_hba.conf
   echo "host all readonly YOUR_APP_IP/32 md5" >> /var/lib/postgresql/data/pg_hba.conf
   
   exit
   ```
   
   Restart to apply:
   ```bash
   sudo docker compose restart musicbrainz-db
   ```

3. **Configure firewall to allow application access**
   ```bash
   # Allow PostgreSQL from specific application server IPs
   sudo ufw allow from YOUR_APP_SERVER_IP to any port 5432
   ```

4. **Test remote connection**
   
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
sudo docker compose exec musicbrainz admin/replication-status
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
sudo docker compose exec -T musicbrainz-db pg_dump -U musicbrainz musicbrainz_db | \
  gzip > musicbrainz-backup-$(date +%Y%m%d).sql.gz
```

**Automate backups with cron:**
```bash
# /etc/cron.daily/musicbrainz-backup
#!/bin/bash
BACKUP_DIR=/backups/musicbrainz
mkdir -p $BACKUP_DIR
cd /home/YOUR_USER/musicbrainz-docker
docker compose exec -T musicbrainz-db pg_dump -U musicbrainz musicbrainz_db | \
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

### Replication falling behind
```bash
# Check status
sudo docker compose exec musicbrainz admin/replication-status

# Manually trigger replication
sudo docker compose exec musicbrainz admin/replication-up
```

### Out of disk space
```bash
# Check Docker disk usage
sudo docker system df

# Clean up old images and containers
sudo docker system prune -a

# Check database size
sudo docker compose exec musicbrainz-db psql -U musicbrainz -d musicbrainz_db \
  -c "SELECT pg_size_pretty(pg_database_size('musicbrainz_db'));"
```

### Can't connect remotely
```bash
# Check PostgreSQL is listening
sudo docker compose exec musicbrainz-db psql -U musicbrainz -d musicbrainz_db \
  -c "SHOW listen_addresses;"

# Check firewall
sudo ufw status

# Check pg_hba.conf
sudo docker compose exec musicbrainz-db cat /var/lib/postgresql/data/pg_hba.conf
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
