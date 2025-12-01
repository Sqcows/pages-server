#!/usr/bin/env python3
"""
Forgejo Pages Server - Cache Reaper Script

This script connects to Redis and removes stale domain mappings for repositories
that no longer have a .pages file. It should be run periodically via cron.

Usage:
    python reaper.py --redis-host localhost --redis-port 6379 --forgejo-host https://git.example.com

Environment variables (alternative to CLI args):
    REDIS_HOST, REDIS_PORT, REDIS_PASSWORD, FORGEJO_HOST, FORGEJO_TOKEN
"""

import argparse
import os
import sys
import time
import redis
import requests
from typing import Optional, Tuple, List


class CacheReaper:
    """Cleans up stale domain mappings from Redis cache."""

    def __init__(
        self,
        redis_host: str,
        redis_port: int,
        redis_password: Optional[str],
        forgejo_host: str,
        forgejo_token: Optional[str],
        dry_run: bool = False,
    ):
        self.redis_host = redis_host
        self.redis_port = redis_port
        self.redis_password = redis_password
        self.forgejo_host = forgejo_host.rstrip("/")
        self.forgejo_token = forgejo_token
        self.dry_run = dry_run

        # Connect to Redis
        self.redis_client = redis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password,
            decode_responses=True,
        )

        # Setup session for Forgejo API calls
        self.session = requests.Session()
        if forgejo_token:
            self.session.headers.update({"Authorization": f"token {forgejo_token}"})
        self.session.headers.update({"Accept": "application/json"})

    def has_pages_file(self, username: str, repository: str) -> bool:
        """Check if a repository has a .pages file via Forgejo API."""
        url = f"{self.forgejo_host}/api/v1/repos/{username}/{repository}/contents/.pages"

        try:
            response = self.session.get(url, timeout=10)
            return response.status_code == 200
        except requests.RequestException as e:
            print(f"  ⚠️  Error checking {username}/{repository}: {e}")
            # On error, assume it still exists (don't delete)
            return True

    def parse_repo_mapping(self, value: str) -> Optional[Tuple[str, str]]:
        """Parse 'username:repository' string into tuple."""
        parts = value.split(":", 1)
        if len(parts) == 2:
            return parts[0], parts[1]
        return None

    def sanitize_domain_name(self, domain: str) -> str:
        """Sanitize domain name for Traefik router name (matches Go implementation)."""
        # Replace dots and other special characters with hyphens
        sanitized = domain.replace(".", "-").replace("_", "-")
        # Remove any other non-alphanumeric characters except hyphens
        sanitized = "".join(c if c.isalnum() or c == "-" else "-" for c in sanitized)
        return sanitized

    def delete_domain_mappings(self, domain: str, username: str, repository: str):
        """Delete all cache entries for a domain mapping."""
        keys_to_delete = []

        # Forward mapping: custom_domain:domain
        forward_key = f"custom_domain:{domain}"
        keys_to_delete.append(forward_key)

        # Reverse mapping: username:repository
        reverse_key = f"{username}:{repository}"
        keys_to_delete.append(reverse_key)

        # Traefik router configuration keys
        sanitized_domain = self.sanitize_domain_name(domain)
        traefik_keys = [
            f"traefik/http/routers/custom-{sanitized_domain}/rule",
            f"traefik/http/routers/custom-{sanitized_domain}/entrypoints/0",
            f"traefik/http/routers/custom-{sanitized_domain}/service",
            f"traefik/http/routers/custom-{sanitized_domain}/tls/certresolver",
            f"traefik/http/routers/custom-{sanitized_domain}/middlewares/0",
            f"traefik/http/routers/custom-{sanitized_domain}/priority",
        ]
        keys_to_delete.extend(traefik_keys)

        if self.dry_run:
            print(f"  🔍 [DRY RUN] Would delete {len(keys_to_delete)} keys:")
            for key in keys_to_delete:
                print(f"     - {key}")
        else:
            deleted_count = 0
            for key in keys_to_delete:
                try:
                    if self.redis_client.delete(key) > 0:
                        deleted_count += 1
                        print(f"     ✓ Deleted: {key}")
                except redis.RedisError as e:
                    print(f"     ✗ Failed to delete {key}: {e}")

            print(f"  ✓ Deleted {deleted_count}/{len(keys_to_delete)} keys")

    def scan_and_clean(self) -> Tuple[int, int, int]:
        """
        Scan Redis for custom domain mappings and clean up stale entries.

        Returns:
            Tuple of (total_domains, cleaned_domains, error_count)
        """
        print(f"\n🔍 Scanning Redis at {self.redis_host}:{self.redis_port}")
        print(f"🌐 Forgejo API: {self.forgejo_host}")
        if self.dry_run:
            print("🔍 DRY RUN MODE - No changes will be made\n")
        else:
            print("")

        total_domains = 0
        cleaned_domains = 0
        error_count = 0

        # Scan for all custom_domain:* keys
        cursor = 0
        pattern = "custom_domain:*"

        while True:
            cursor, keys = self.redis_client.scan(
                cursor=cursor, match=pattern, count=100
            )

            for key in keys:
                total_domains += 1
                # Extract domain from key (remove "custom_domain:" prefix)
                domain = key[len("custom_domain:") :]

                # Get the repository mapping
                value = self.redis_client.get(key)
                if not value:
                    print(f"⚠️  {domain}: No value found (skipping)")
                    error_count += 1
                    continue

                # Parse username:repository
                repo_tuple = self.parse_repo_mapping(value)
                if not repo_tuple:
                    print(f"⚠️  {domain}: Invalid mapping format '{value}' (skipping)")
                    error_count += 1
                    continue

                username, repository = repo_tuple
                print(f"📋 {domain} -> {username}/{repository}")

                # Check if repository still has .pages file
                if not self.has_pages_file(username, repository):
                    print(f"  ❌ Repository no longer has .pages file")
                    self.delete_domain_mappings(domain, username, repository)
                    cleaned_domains += 1
                else:
                    print(f"  ✓ Repository still has .pages file")

            if cursor == 0:
                break

        return total_domains, cleaned_domains, error_count

    def print_summary(self, total: int, cleaned: int, errors: int, duration: float):
        """Print summary of reaper run."""
        print("\n" + "=" * 60)
        print("📊 REAPER SUMMARY")
        print("=" * 60)
        print(f"Total domains scanned:  {total}")
        print(f"Stale domains cleaned:  {cleaned}")
        print(f"Errors encountered:     {errors}")
        print(f"Duration:               {duration:.2f} seconds")
        if self.dry_run:
            print("\n🔍 DRY RUN - No actual changes were made")
        print("=" * 60 + "\n")


def main():
    parser = argparse.ArgumentParser(
        description="Forgejo Pages Server cache reaper - cleans up stale domain mappings"
    )
    parser.add_argument(
        "--redis-host",
        default=os.getenv("REDIS_HOST", "localhost"),
        help="Redis host (default: localhost or $REDIS_HOST)",
    )
    parser.add_argument(
        "--redis-port",
        type=int,
        default=int(os.getenv("REDIS_PORT", "6379")),
        help="Redis port (default: 6379 or $REDIS_PORT)",
    )
    parser.add_argument(
        "--redis-password",
        default=os.getenv("REDIS_PASSWORD"),
        help="Redis password (default: $REDIS_PASSWORD)",
    )
    parser.add_argument(
        "--forgejo-host",
        default=os.getenv("FORGEJO_HOST"),
        required=not os.getenv("FORGEJO_HOST"),
        help="Forgejo host URL (required, or set $FORGEJO_HOST)",
    )
    parser.add_argument(
        "--forgejo-token",
        default=os.getenv("FORGEJO_TOKEN"),
        help="Forgejo API token (default: $FORGEJO_TOKEN)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Dry run mode - show what would be deleted without actually deleting",
    )

    args = parser.parse_args()

    # Create reaper instance
    reaper = CacheReaper(
        redis_host=args.redis_host,
        redis_port=args.redis_port,
        redis_password=args.redis_password,
        forgejo_host=args.forgejo_host,
        forgejo_token=args.forgejo_token,
        dry_run=args.dry_run,
    )

    # Test Redis connection
    try:
        reaper.redis_client.ping()
        print("✓ Redis connection successful")
    except redis.ConnectionError as e:
        print(f"✗ Failed to connect to Redis: {e}")
        sys.exit(1)

    # Run the reaper
    start_time = time.time()
    try:
        total, cleaned, errors = reaper.scan_and_clean()
        duration = time.time() - start_time
        reaper.print_summary(total, cleaned, errors, duration)

        # Exit with appropriate code
        if errors > 0:
            sys.exit(2)  # Partial success
        sys.exit(0)  # Success

    except KeyboardInterrupt:
        print("\n\n⚠️  Interrupted by user")
        sys.exit(130)
    except Exception as e:
        print(f"\n\n✗ Fatal error: {e}")
        import traceback

        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
