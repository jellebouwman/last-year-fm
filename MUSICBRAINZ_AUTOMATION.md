# MusicBrainz Infrastructure Automation

## Overview

This document describes how to make the MusicBrainz VPS setup reproducible and maintainable using Infrastructure as Code (IaC) principles. Instead of manual edits and documentation, we use version-controlled configuration files and automation tools.

## Current Pain Points (Manual Setup)

1. **Manual file edits inside containers** - Hard to reproduce, not version controlled
2. **Imperative commands** - Must remember exact sequence
3. **Documentation drift** - README can get out of sync with reality
4. **No disaster recovery** - If VPS dies, manual rebuild required
5. **Configuration scattered** - Some in containers, some in compose files, some in shell history

## Recommended Approach

**Docker Compose Override + Ansible (optional) + Shell Scripts**

### Why This Approach?

- **Lightweight**: Builds on existing Docker Compose setup
- **Version controlled**: All config in git
- **Reproducible**: Can rebuild from scratch
- **Simple**: No complex tools required
- **Flexible**: Can add Ansible later if needed

## File Structure

```
musicbrainz-docker/
├── docker-compose.yml                # Base (from upstream repo)
├── docker-compose.override.yml       # Our customizations (git)
├── crons.conf                        # Replication schedule (git)
├── pg_hba_custom.conf                # PostgreSQL access rules (git)
├── scripts/
│   ├── bootstrap-vps.sh              # Initial VPS setup
│   ├── create-readonly-user.sh       # Database user creation
│   └── setup-firewall.sh             # UFW configuration
├── .env                              # Environment variables (git-ignored)
└── README.md                         # Setup instructions
```

## Implementation Steps

### Step 1: Create docker-compose.override.yml

This file contains all our customizations to the base setup:

```yaml
services:
  db:
    # Expose PostgreSQL to host network
    ports:
      - "5432:5432"

    # Mount custom pg_hba.conf for access control
    volumes:
      - ./pg_hba_custom.conf:/var/lib/postgresql/data/pg_hba.conf

  musicbrainz:
    # Mount crons.conf so it's version controlled
    volumes:
      - ./crons.conf:/crons.conf:ro
```

**Benefits:**
- Port exposure is declarative
- Cron config is in git, not manually copied
- PostgreSQL access rules are version controlled
- No manual `docker compose cp` commands needed

### Step 2: Create crons.conf

**File: `crons.conf`**
```
0 * * * * /musicbrainz-server/admin/cron/mirror.sh
```

**Status:** ✅ Created (2025-11-17)

### Step 3: Create pg_hba_custom.conf

**Purpose:** Define which IPs can connect to PostgreSQL

**File: `pg_hba_custom.conf`**
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust

# Allow admin user from Docker network
host    all             musicbrainz     all                     md5

# Allow readonly user from specific IPs (replace with your app server IPs)
host    musicbrainz_db  readonly        YOUR_APP_SERVER_IP/32   md5
host    musicbrainz_db  readonly        YOUR_DEV_MACHINE_IP/32  md5
```

**Status:** 🔄 To be created (Phase 5)

### Step 4: Create Database Setup Script

**File: `scripts/create-readonly-user.sh`**
```bash
#!/bin/bash
# Creates read-only database user for applications

set -e

echo "Creating read-only database user..."

docker compose exec -T db psql -U musicbrainz -d musicbrainz_db << 'EOF'
-- Create read-only user
CREATE USER IF NOT EXISTS readonly WITH PASSWORD 'CHANGE_THIS_PASSWORD';

-- Grant connection
GRANT CONNECT ON DATABASE musicbrainz_db TO readonly;

-- Grant usage on schema
GRANT USAGE ON SCHEMA musicbrainz TO readonly;

-- Grant SELECT on all tables
GRANT SELECT ON ALL TABLES IN SCHEMA musicbrainz TO readonly;

-- Grant SELECT on future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA musicbrainz
  GRANT SELECT ON TABLES TO readonly;

-- Verify
\du readonly
EOF

echo "✅ Read-only user created successfully"
```

**Status:** 🔄 To be created (Phase 5)

### Step 5: Create VPS Bootstrap Script

**File: `scripts/bootstrap-vps.sh`**
```bash
#!/bin/bash
# Run this on a fresh VPS to prepare it for MusicBrainz

set -e

echo "🚀 Bootstrapping VPS for MusicBrainz..."

# Update system
echo "📦 Updating system packages..."
sudo apt update && sudo apt upgrade -y

# Install Docker
echo "🐳 Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    rm get-docker.sh
fi

# Install Docker Compose plugin
echo "🔧 Installing Docker Compose..."
sudo apt install -y docker-compose-plugin

# Add current user to docker group
echo "👤 Adding user to docker group..."
sudo usermod -aG docker $USER

# Install utilities
echo "🛠️  Installing utilities..."
sudo apt install -y vim git ufw

# Configure firewall
echo "🔥 Configuring firewall..."
sudo ufw allow 22/tcp comment 'SSH'
sudo ufw allow 5432/tcp comment 'PostgreSQL'
sudo ufw --force enable

# Verify installations
echo ""
echo "✅ Bootstrap complete!"
echo ""
echo "Installed versions:"
docker --version
docker compose version
echo ""
echo "⚠️  You may need to log out and back in for docker group changes to take effect"
echo ""
echo "Next steps:"
echo "1. Clone musicbrainz-docker repository"
echo "2. Copy your customized docker-compose.override.yml, crons.conf, etc."
echo "3. Run: docker compose up -d"
echo "4. Run: docker compose run --rm musicbrainz createdb.sh -fetch"
```

**Status:** 🔄 To be created

### Step 6: Create Firewall Configuration Script

**File: `scripts/setup-firewall.sh`**
```bash
#!/bin/bash
# Configure UFW firewall for MusicBrainz VPS

set -e

# Read IPs from environment or arguments
APP_SERVER_IP=${1:-$APP_SERVER_IP}
DEV_MACHINE_IP=${2:-$DEV_MACHINE_IP}

echo "🔥 Configuring firewall..."

# Basic rules
sudo ufw allow 22/tcp comment 'SSH'

# PostgreSQL access
if [ -n "$APP_SERVER_IP" ]; then
    sudo ufw allow from $APP_SERVER_IP to any port 5432 comment 'App Server'
fi

if [ -n "$DEV_MACHINE_IP" ]; then
    sudo ufw allow from $DEV_MACHINE_IP to any port 5432 comment 'Dev Machine'
fi

# Enable firewall
sudo ufw --force enable

# Show status
sudo ufw status numbered

echo "✅ Firewall configured"
```

**Status:** 🔄 To be created

## Usage

### Fresh VPS Setup (Complete Rebuild)

```bash
# 1. On fresh VPS, bootstrap the environment
curl -fsSL https://raw.githubusercontent.com/YOUR_REPO/main/scripts/bootstrap-vps.sh | bash

# Log out and back in for docker group to take effect
exit

# 2. Clone repository
git clone https://github.com/metabrainz/musicbrainz-docker
cd musicbrainz-docker

# 3. Clone your config repository (or copy files)
git clone https://github.com/YOUR_REPO/musicbrainz-config config
cp config/docker-compose.override.yml .
cp config/crons.conf .
cp config/pg_hba_custom.conf .
cp -r config/scripts .

# 4. Configure environment variables
cp default.env .env
# Edit .env if needed

# 5. Start services
docker compose up -d

# 6. Import database (4-8 hours)
docker compose run --rm musicbrainz createdb.sh -fetch

# 7. Create read-only user
bash scripts/create-readonly-user.sh

# 8. Configure firewall
bash scripts/setup-firewall.sh YOUR_APP_IP YOUR_DEV_IP

# 9. Verify
docker compose ps
docker compose exec db psql -U readonly -d musicbrainz_db -c "SELECT COUNT(*) FROM musicbrainz.artist;"
```

### Updating Configuration

```bash
# Edit config files
vim crons.conf
vim docker-compose.override.yml

# Commit to git
git add .
git commit -m "Update replication schedule"
git push

# Apply changes
docker compose down
docker compose up -d
```

## Advanced: Ansible Playbook (Optional)

For managing multiple servers or completely automated provisioning:

**File: `ansible/playbook.yml`**
```yaml
---
- name: Setup MusicBrainz VPS
  hosts: musicbrainz_servers
  become: yes

  vars:
    musicbrainz_dir: /home/{{ ansible_user }}/musicbrainz-docker
    readonly_password: "{{ lookup('env', 'READONLY_PASSWORD') }}"

  tasks:
    - name: Install Docker
      shell: curl -fsSL https://get.docker.com | sh
      args:
        creates: /usr/bin/docker

    - name: Install Docker Compose
      apt:
        name: docker-compose-plugin
        state: present

    - name: Clone musicbrainz-docker
      git:
        repo: https://github.com/metabrainz/musicbrainz-docker
        dest: "{{ musicbrainz_dir }}"

    - name: Copy docker-compose.override.yml
      copy:
        src: ../docker-compose.override.yml
        dest: "{{ musicbrainz_dir }}/docker-compose.override.yml"

    - name: Copy crons.conf
      copy:
        src: ../crons.conf
        dest: "{{ musicbrainz_dir }}/crons.conf"

    - name: Copy pg_hba_custom.conf
      template:
        src: ../pg_hba_custom.conf.j2
        dest: "{{ musicbrainz_dir }}/pg_hba_custom.conf"

    - name: Start services
      command: docker compose up -d
      args:
        chdir: "{{ musicbrainz_dir }}"

    - name: Configure firewall
      ufw:
        rule: allow
        port: "{{ item.port }}"
        from_ip: "{{ item.ip }}"
      loop:
        - { port: 5432, ip: "{{ app_server_ip }}" }
        - { port: 5432, ip: "{{ dev_machine_ip }}" }
```

**Status:** 📋 Future consideration

## Comparison: Manual vs Automated

| Aspect | Manual (Current) | Automated (This Doc) |
|--------|-----------------|---------------------|
| **Setup time** | 1 hour active + waiting | 10 min active + waiting |
| **Reproducible?** | Requires following docs | One command |
| **Version control** | Only docs | All config files |
| **Disaster recovery** | Manual rebuild | Script rebuild |
| **Configuration drift** | Easy to happen | Prevented |
| **Team collaboration** | Hard (tribal knowledge) | Easy (code review) |
| **Testing changes** | Risky (prod only) | Can test locally |

## Migration Path (From Manual to Automated)

Since we already have a running setup, we'll extract the configuration incrementally:

- [x] **Step 1:** Create `crons.conf` file (✅ Completed 2025-11-17)
- [x] **Step 2:** Create `docker-compose.override.yml` with current port mappings (✅ Completed 2025-11-17)
- [x] **Step 3:** Configure `.env` for correct compose file chain (✅ Completed 2025-11-17)
- [x] **Step 4:** Create read-only database user (✅ Completed 2025-11-17)
- [x] **Step 5:** Configure firewall (✅ Completed 2025-11-17)
- [ ] **Step 6:** Create automation scripts
- [ ] **Step 7:** Test on a temporary VPS to verify reproducibility
- [ ] **Step 8:** Commit all files to git

## Actual Implementation (Lessons Learned)

### Phase 5 Completion (2025-11-17)

**What We Actually Did:**

1. **Created read-only database user**
   ```sql
   CREATE USER readonly WITH PASSWORD 'strong_password';
   GRANT CONNECT ON DATABASE musicbrainz_db TO readonly;
   GRANT USAGE ON SCHEMA musicbrainz TO readonly;
   GRANT SELECT ON ALL TABLES IN SCHEMA musicbrainz TO readonly;
   ALTER DEFAULT PRIVILEGES IN SCHEMA musicbrainz GRANT SELECT ON TABLES TO readonly;
   ```

2. **Verified PostgreSQL was already configured for remote connections**
   - `pg_hba.conf` already had `host all all all scram-sha-256`
   - `listen_addresses` already set to `*`
   - No manual edits needed! ✅

3. **Created `docker-compose.override.yml`**
   ```yaml
   services:
     db:
       ports:
         - "5432:5432"

     musicbrainz:
       volumes:
         - ./crons.conf:/crons.conf:ro
   ```

4. **Fixed `.env` file** (CRITICAL GOTCHA!)
   - Original: `COMPOSE_FILE=docker-compose.alt.db-only-mirror.yml`
   - Fixed: `COMPOSE_FILE=docker-compose.alt.db-only-mirror.yml:docker-compose.override.yml`
   - **Gotcha**: Docker Compose needs the override file explicitly listed when using custom COMPOSE_FILE
   - **Impact**: Without this, the override is completely ignored!

5. **Removed unwanted containers**
   - First compose up loaded ALL services (indexer, mq, search) from base docker-compose.yml
   - Solution: Removed `docker-compose.yml` from COMPOSE_FILE chain
   - Final minimal stack: just db + musicbrainz + redis

6. **Configured firewall**
   ```bash
   sudo ufw allow 5432/tcp comment 'PostgreSQL'
   ```

7. **Tested successfully from dev machine**
   ```bash
   psql -h 46.62.240.182 -U readonly -d musicbrainz_db -c "SELECT COUNT(*) FROM musicbrainz.artist;"
   # Result: 2,732,887 artists
   ```

### Key Gotchas and Solutions

#### 1. docker-compose.override.yml Not Working
**Problem:** Override file was ignored even though correctly named
**Root Cause:** `.env` had `COMPOSE_FILE=docker-compose.alt.db-only-mirror.yml` which overrides default behavior
**Solution:** Add override to the chain: `COMPOSE_FILE=docker-compose.alt.db-only-mirror.yml:docker-compose.override.yml`

#### 2. Extra Containers Starting
**Problem:** indexer, mq, search containers started (not needed for db-only)
**Root Cause:** Including `docker-compose.yml` in the chain loaded full MusicBrainz stack
**Solution:** Only include alt.db-only-mirror + override in COMPOSE_FILE

#### 3. YAML Indentation Issues
**Problem:** Used tabs instead of spaces in YAML
**Solution:** Use vim, ensure 2 spaces per indentation level (no tabs)

#### 4. Password Syntax Error in psql
**Problem:** `CREATE USER readonly WITH PASSWORD _password` failed
**Root Cause:** Password not quoted
**Solution:** `CREATE USER readonly WITH PASSWORD '_password'`

### Critical Files for Reproducibility

**1. `crons.conf`** (already created)
```
0 * * * * /musicbrainz-server/admin/cron/mirror.sh
```

**2. `docker-compose.override.yml`** (already created)
```yaml
services:
  db:
    ports:
      - "5432:5432"

  musicbrainz:
    volumes:
      - ./crons.conf:/crons.conf:ro
```

**3. `.env`** (must modify)
```bash
COMPOSE_FILE=docker-compose.alt.db-only-mirror.yml:docker-compose.override.yml
```

**4. Database user creation** (manual for now)
- See SQL commands above
- Future: Create script in `scripts/create-readonly-user.sh`

**5. Firewall rules** (manual for now)
```bash
sudo ufw allow 22/tcp comment 'SSH'
sudo ufw allow 5432/tcp comment 'PostgreSQL'
```

### Production Readiness Checklist

- [x] Read-only database user created
- [x] Remote connections working
- [x] Firewall configured
- [x] Minimal container setup (db + replication only)
- [x] Hourly replication configured
- [ ] SSL/TLS for PostgreSQL connections
- [ ] IP-restricted firewall rules (currently open to all)
- [ ] Monitoring and alerting
- [ ] Backup automation
- [ ] Documentation in git

### Connection Details for Applications

```typescript
// Node.js / TypeScript example
import { Pool } from 'pg';

const pool = new Pool({
  host: '46.62.240.182',
  port: 5432,
  database: 'musicbrainz_db',
  user: 'readonly',
  password: process.env.MUSICBRAINZ_PASSWORD, // from 1Password
  max: 20,
  idleTimeoutMillis: 30000,
});

// Test query
const result = await pool.query('SELECT COUNT(*) FROM musicbrainz.artist');
console.log(`Total artists: ${result.rows[0].count}`);
```

---

**Last Updated:** 2025-11-17
**Status:** Phase 5 Complete ✅ - Ready for Application Integration
