# Forgejo Pages Server - Cache Reaper

The cache reaper is a Python script that cleans up stale custom domain mappings from Redis. It should be run periodically via cron to ensure that domains pointing to deleted or reconfigured repositories are removed from the cache.

## What It Does

The reaper script:

1. **Scans Redis** for all custom domain mappings (`custom_domain:*` keys)
2. **Checks each repository** via Forgejo API to see if it still has a `.pages` file
3. **Removes stale mappings** when a repository no longer has a `.pages` file:
   - Forward mapping: `custom_domain:{domain}`
   - Reverse mapping: `{username}:{repository}`
   - Traefik router configurations: `traefik/http/routers/custom-{domain}/*`

## Installation

### Prerequisites

- Python 3.7 or later
- Access to the Redis instance used by the plugin
- (Optional) Forgejo API token for private repositories

### Install Dependencies

```bash
cd reaper
pip install -r requirements.txt
```

Or using a virtual environment:

```bash
cd reaper
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt
```

## Usage

### Command Line Arguments

```bash
python reaper.py --redis-host localhost \
                 --redis-port 6379 \
                 --redis-password mypassword \
                 --forgejo-host https://git.example.com \
                 --forgejo-token my-api-token \
                 --dry-run
```

### Environment Variables

You can also use environment variables instead of command-line arguments:

```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=mypassword
export FORGEJO_HOST=https://git.example.com
export FORGEJO_TOKEN=my-api-token

python reaper.py --dry-run
```

### Options

| Option | Environment Variable | Description | Required | Default |
|--------|---------------------|-------------|----------|---------|
| `--redis-host` | `REDIS_HOST` | Redis server hostname | No | `localhost` |
| `--redis-port` | `REDIS_PORT` | Redis server port | No | `6379` |
| `--redis-password` | `REDIS_PASSWORD` | Redis password | No | None |
| `--forgejo-host` | `FORGEJO_HOST` | Forgejo host URL | Yes | None |
| `--forgejo-token` | `FORGEJO_TOKEN` | Forgejo API token | No* | None |
| `--dry-run` | N/A | Show what would be deleted without deleting | No | `false` |

*API token is required if you need to check private repositories

## Dry Run Mode

**Always test with `--dry-run` first** to see what would be deleted:

```bash
python reaper.py --redis-host localhost \
                 --forgejo-host https://git.example.com \
                 --dry-run
```

Example output:
```
🔍 Scanning Redis at localhost:6379
🌐 Forgejo API: https://git.example.com
🔍 DRY RUN MODE - No changes will be made

📋 example.com -> user1/repo1
  ❌ Repository no longer has .pages file
  🔍 [DRY RUN] Would delete 7 keys:
     - custom_domain:example.com
     - user1:repo1
     - traefik/http/routers/custom-example-com/rule
     - traefik/http/routers/custom-example-com/entrypoints/0
     - traefik/http/routers/custom-example-com/service
     - traefik/http/routers/custom-example-com/tls/certresolver
     - traefik/http/routers/custom-example-com/middlewares/0

📋 squarecows.com -> squarecows/sqcows-web
  ✓ Repository still has .pages file

============================================================
📊 REAPER SUMMARY
============================================================
Total domains scanned:  2
Stale domains cleaned:  1
Errors encountered:     0
Duration:               1.23 seconds

🔍 DRY RUN - No actual changes were made
============================================================
```

## Production Usage

Once you've tested with `--dry-run`, remove the flag to actually delete stale entries:

```bash
python reaper.py --redis-host localhost \
                 --forgejo-host https://git.example.com \
                 --forgejo-token my-api-token
```

## Scheduling with Cron

Run the reaper automatically using cron. Edit your crontab:

```bash
crontab -e
```

Add one of these entries:

### Run every hour
```cron
0 * * * * /usr/bin/python3 /path/to/reaper/reaper.py --redis-host localhost --forgejo-host https://git.example.com >> /var/log/pages-reaper.log 2>&1
```

### Run every 6 hours
```cron
0 */6 * * * /usr/bin/python3 /path/to/reaper/reaper.py --redis-host localhost --forgejo-host https://git.example.com >> /var/log/pages-reaper.log 2>&1
```

### Run daily at 3 AM
```cron
0 3 * * * /usr/bin/python3 /path/to/reaper/reaper.py --redis-host localhost --forgejo-host https://git.example.com >> /var/log/pages-reaper.log 2>&1
```

### Using environment variables (recommended)

Create a shell script `/path/to/reaper/run-reaper.sh`:

```bash
#!/bin/bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=mypassword
export FORGEJO_HOST=https://git.example.com
export FORGEJO_TOKEN=my-api-token

cd /path/to/reaper
/usr/bin/python3 reaper.py
```

Make it executable:
```bash
chmod +x /path/to/reaper/run-reaper.sh
```

Add to crontab:
```cron
0 * * * * /path/to/reaper/run-reaper.sh >> /var/log/pages-reaper.log 2>&1
```

## Exit Codes

- `0`: Success - all domains processed without errors
- `1`: Fatal error (e.g., can't connect to Redis, unexpected exception)
- `2`: Partial success - some domains processed with errors
- `130`: Interrupted by user (Ctrl+C)

## Security Considerations

1. **API Token**: Store your Forgejo API token securely:
   - Use environment variables instead of command-line arguments
   - Restrict file permissions on the run script: `chmod 700 run-reaper.sh`
   - Use a dedicated API token with minimal permissions (read-only access to repositories)

2. **Redis Password**: If Redis requires authentication:
   - Use `REDIS_PASSWORD` environment variable
   - Secure the run script with appropriate file permissions

3. **Network Access**: Ensure the reaper script can reach:
   - Redis server (typically port 6379)
   - Forgejo API (typically port 443 for HTTPS)

## Troubleshooting

### Connection Errors

If you get Redis connection errors:
```
✗ Failed to connect to Redis: Error 111 connecting to localhost:6379. Connection refused.
```

Check:
- Redis is running: `redis-cli ping`
- Correct host/port configuration
- Redis password if required
- Firewall rules allow connection

### API Errors

If you get Forgejo API errors:
```
⚠️  Error checking user1/repo1: 401 Unauthorized
```

Check:
- Forgejo host URL is correct (include `https://`)
- API token is valid and has correct permissions
- Repository is accessible with the token

### Permission Denied

If you get permission errors when running the script:
```bash
chmod +x reaper.py
python reaper.py --help
```

## Monitoring

Monitor the reaper's effectiveness:

1. **Check logs** for errors and cleanup counts
2. **Watch Redis keys** to ensure stale entries are removed:
   ```bash
   redis-cli --scan --pattern "custom_domain:*" | wc -l
   ```
3. **Set up alerts** if error count is consistently high

## Example Run Output

Successful run:
```
✓ Redis connection successful

🔍 Scanning Redis at localhost:6379
🌐 Forgejo API: https://git.example.com

📋 example.com -> user1/old-repo
  ❌ Repository no longer has .pages file
     ✓ Deleted: custom_domain:example.com
     ✓ Deleted: user1:old-repo
     ✓ Deleted: traefik/http/routers/custom-example-com/rule
     ✓ Deleted: traefik/http/routers/custom-example-com/entrypoints/0
     ✓ Deleted: traefik/http/routers/custom-example-com/service
     ✓ Deleted: traefik/http/routers/custom-example-com/tls/certresolver
     ✓ Deleted: traefik/http/routers/custom-example-com/middlewares/0
  ✓ Deleted 7/7 keys

📋 squarecows.com -> squarecows/sqcows-web
  ✓ Repository still has .pages file

============================================================
📊 REAPER SUMMARY
============================================================
Total domains scanned:  2
Stale domains cleaned:  1
Errors encountered:     0
Duration:               1.45 seconds
============================================================
```

## Support

For issues or questions:
- Check the [main repository documentation](../README.md)
- Report bugs at: https://code.squarecows.com/SquareCows/pages-server/issues
