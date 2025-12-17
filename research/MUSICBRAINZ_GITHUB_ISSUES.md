# MusicBrainz VPS - GitHub Issues

Copy-paste these into your GitHub repository as issues.

---

## Issue 1: Add SSL/TLS encryption for PostgreSQL connections

**Labels:** `security`, `enhancement`, `infrastructure`

**Description:**

Currently, PostgreSQL connections are unencrypted. For production use, we should enable SSL/TLS to protect data in transit.

**Tasks:**
- [ ] Generate SSL certificates (Let's Encrypt or self-signed for internal use)
- [ ] Configure PostgreSQL to require SSL connections
- [ ] Update `docker-compose.override.yml` to mount SSL certificates
- [ ] Update application connection strings to use SSL
- [ ] Test connection with SSL enabled
- [ ] Update documentation in `MUSICBRAINZ_VPS_SETUP.md`

**Resources:**
- [PostgreSQL SSL Documentation](https://www.postgresql.org/docs/current/ssl-tcp.html)
- Docker volume mounts for certificates

**Priority:** Medium (higher if exposing to public internet)

---

## Issue 2: Restrict PostgreSQL firewall to specific IP addresses

**Labels:** `security`, `infrastructure`

**Description:**

Currently, PostgreSQL port 5432 is open to all IP addresses (`0.0.0.0/0`). We should restrict access to only known application servers and development machines.

**Current rule:**
```bash
sudo ufw status
# Shows: 5432/tcp ALLOW Anywhere
```

**Tasks:**
- [ ] Identify all IPs that need database access (app servers, dev machines, CI/CD)
- [ ] Remove the broad `5432/tcp ALLOW Anywhere` rule
- [ ] Add specific IP-based rules:
  ```bash
  sudo ufw delete allow 5432/tcp
  sudo ufw allow from APP_SERVER_IP to any port 5432 comment 'App Server'
  sudo ufw allow from DEV_MACHINE_IP to any port 5432 comment 'Dev Machine'
  ```
- [ ] Test connections still work from allowed IPs
- [ ] Verify blocked from unauthorized IPs
- [ ] Update `scripts/setup-firewall.sh` with IP-based rules
- [ ] Document IP whitelist in repository

**Priority:** High (security best practice)

---

## Issue 3: Set up monitoring and alerting for MusicBrainz VPS

**Labels:** `monitoring`, `devops`, `infrastructure`

**Description:**

We need visibility into the health and performance of the MusicBrainz database to catch issues before they impact applications.

**Monitoring needs:**
- Container health (db, musicbrainz, redis)
- Disk space usage (database can grow large)
- Replication status (is it running? falling behind?)
- Query performance
- Connection pool status
- System resources (CPU, RAM, network)

**Tasks:**
- [ ] Choose monitoring solution (options below)
- [ ] Set up metrics collection
- [ ] Create dashboards for key metrics
- [ ] Configure alerts for critical issues:
  - Disk space > 80%
  - Containers restarting
  - Replication failures
  - High query latency
- [ ] Document monitoring setup in repository

**Monitoring Options:**
1. **Simple:** Built-in with `docker stats` + cron script that sends email on issues
2. **Cloud:** Use VPS provider's monitoring (DigitalOcean, Hetzner, etc.)
3. **Self-hosted:** Prometheus + Grafana stack
4. **SaaS:** Datadog, New Relic, etc.

**Priority:** Medium (nice to have, important for production)

---

## Issue 4: Implement automated database backups

**Labels:** `backup`, `infrastructure`, `reliability`

**Description:**

While we have replication configured, we don't have backups. If data gets corrupted or accidentally deleted, we need a way to restore.

**Current state:**
- Hourly replication from MusicBrainz (keeps us in sync)
- No snapshots or backups (can't rollback to previous state)

**Tasks:**
- [ ] Create backup script (`scripts/backup-database.sh`)
  ```bash
  #!/bin/bash
  BACKUP_DIR=/backups/musicbrainz
  mkdir -p $BACKUP_DIR
  cd ~/musicbrainz-docker
  docker compose exec -T db pg_dump -U musicbrainz musicbrainz_db | \
    gzip > $BACKUP_DIR/musicbrainz-$(date +%Y%m%d-%H%M).sql.gz
  ```
- [ ] Set up cron job for automated backups (daily or weekly)
- [ ] Configure backup retention policy (keep last 7 daily, 4 weekly, 12 monthly?)
- [ ] Store backups off-server (S3, Backblaze B2, rsync to another server)
- [ ] Test backup restoration process
- [ ] Document backup/restore procedures
- [ ] Add monitoring for backup success/failure

**Considerations:**
- Full database is ~100GB, backups will be large
- Consider incremental backups or snapshots
- Alternatively, just keep track of replication sequence numbers to re-import from official dumps

**Priority:** Medium (disaster recovery planning)

---

## Issue 5: Create automation scripts for reproducible setup

**Labels:** `automation`, `documentation`, `infrastructure`

**Description:**

We have `MUSICBRAINZ_AUTOMATION.md` documenting the ideal setup, but haven't created the actual automation scripts yet. This would make rebuilding the VPS or setting up a staging environment much easier.

**Tasks:**
- [ ] Create `scripts/bootstrap-vps.sh` (install Docker, configure firewall)
- [ ] Create `scripts/create-readonly-user.sh` (automated database user creation)
- [ ] Create `scripts/setup-firewall.sh` (IP-based firewall rules)
- [ ] Create `scripts/health-check.sh` (verify everything is working)
- [ ] Test scripts on a fresh VPS to verify they work
- [ ] Add CI/CD to validate scripts don't break
- [ ] Document script usage in README

**Reference:**
See `MUSICBRAINZ_AUTOMATION.md` for detailed script content

**Priority:** Low (nice to have, helpful for future rebuilds)

---

## Issue 6: Set up staging environment for testing schema updates

**Labels:** `infrastructure`, `testing`

**Description:**

MusicBrainz occasionally releases schema updates. Before applying these to production, we should test them in a staging environment to ensure our queries still work.

**Tasks:**
- [ ] Decide on staging approach:
  - Option A: Separate smaller VPS with recent database dump
  - Option B: Local Docker setup with sample data
  - Option C: Temporary VPS spun up only when needed
- [ ] Document staging environment setup
- [ ] Create checklist for testing schema updates
- [ ] Subscribe to MusicBrainz schema change announcements
- [ ] Document update/testing workflow

**Priority:** Low (schema changes are infrequent, ~2-3 times per year)

---

## Issue 7: Optimize PostgreSQL configuration for read-heavy workloads

**Labels:** `performance`, `optimization`, `infrastructure`

**Description:**

The default PostgreSQL configuration is general-purpose. Since we're using this as a read-only service for applications, we can optimize settings for better query performance.

**Tasks:**
- [ ] Analyze current query patterns from applications
- [ ] Tune PostgreSQL parameters:
  - `shared_buffers` (currently 2GB, may need adjustment)
  - `effective_cache_size`
  - `random_page_cost`
  - `work_mem`
  - `maintenance_work_mem`
- [ ] Add indexes for common queries (if not already present)
- [ ] Monitor query performance before/after changes
- [ ] Document optimized configuration in repository
- [ ] Consider connection pooling (PgBouncer) if needed

**Resources:**
- [PGTune](https://pgtune.leopard.in.ua/) for configuration recommendations
- PostgreSQL performance documentation

**Priority:** Low (optimize when we see performance issues)

---

## Issue 8: Document MusicBrainz schema for application developers

**Labels:** `documentation`, `developer-experience`

**Description:**

Application developers need to understand the MusicBrainz database schema to write effective queries. Create documentation and examples.

**Tasks:**
- [ ] Export schema reference:
  ```bash
  psql -h $MUSICBRAINZ_DB_HOST -U readonly -d musicbrainz_db --schema-only > docs/musicbrainz-schema.sql
  ```
- [ ] Create ERD (Entity Relationship Diagram) of key tables
- [ ] Document common tables and their relationships:
  - `musicbrainz.artist`
  - `musicbrainz.release`
  - `musicbrainz.release_group`
  - `musicbrainz.recording`
  - `musicbrainz.track`
- [ ] Provide example queries for common use cases
- [ ] Add schema documentation to main repository README
- [ ] Set up process to update docs when schema changes

**Priority:** Medium (helps with app development)

---

## Summary

**High Priority:**
- Issue #2: Restrict firewall to specific IPs

**Medium Priority:**
- Issue #1: Add SSL/TLS encryption
- Issue #3: Set up monitoring and alerting
- Issue #4: Implement automated backups
- Issue #8: Document schema for developers

**Low Priority:**
- Issue #5: Create automation scripts
- Issue #6: Set up staging environment
- Issue #7: Optimize PostgreSQL config

**Immediate Next Steps:**
1. Create these issues in your GitHub repository
2. Prioritize based on your needs (production vs development)
3. Tackle high-priority security items first
