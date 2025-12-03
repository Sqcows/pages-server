#!/bin/bash

# Copyright (C) 2025 SquareCows
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

# Generate DNS TXT record verification hash for Bovine Pages Server custom domains
#
# Usage:
#   ./generate-dns-verification-hash.sh <owner> <repository>
#
# Example:
#   ./generate-dns-verification-hash.sh squarecows bovine-website
#
# This script generates the SHA256 hash required for DNS TXT record verification
# when using custom domains with the Bovine Pages Server plugin.

set -e

# Check if required arguments are provided
if [ $# -ne 2 ]; then
    echo "Usage: $0 <owner> <repository>"
    echo ""
    echo "Example: $0 squarecows bovine-website"
    echo ""
    echo "This generates the SHA256 hash for DNS TXT record verification."
    exit 1
fi

OWNER="$1"
REPOSITORY="$2"
REPO_PATH="${OWNER}/${REPOSITORY}"

# Generate SHA256 hash
HASH=$(echo -n "${REPO_PATH}" | shasum -a 256 | awk '{print $1}')

# Display results
echo "==================================================================="
echo "DNS TXT Record Verification Hash Generator"
echo "==================================================================="
echo ""
echo "Repository: ${REPO_PATH}"
echo "Hash:       ${HASH}"
echo ""
echo "Add this DNS TXT record to your custom domain:"
echo ""
echo "  TXT record: bovine-pages-verification=${HASH}"
echo ""
echo "Example (using your DNS provider):"
echo "  Host/Name:  @ (or your subdomain, e.g., www)"
echo "  Type:       TXT"
echo "  Value:      bovine-pages-verification=${HASH}"
echo "  TTL:        3600 (or your DNS provider's default)"
echo ""
echo "==================================================================="
echo ""
echo "To verify your TXT record after DNS propagation:"
echo "  dig TXT yourdomain.com"
echo ""
echo "Or:"
echo "  nslookup -type=TXT yourdomain.com"
echo ""
echo "==================================================================="
