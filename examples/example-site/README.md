# Example Static Site for Forgejo Pages

This directory demonstrates the structure of a repository configured for the Bovine Pages Server plugin.

## Repository Structure

```
example-site/
├── .pages                 # Pages configuration (required)
├── public/                # Static files directory (required)
│   ├── index.html        # Homepage
│   ├── about.html        # About page
│   ├── 404.html          # Custom 404 page (optional)
│   ├── css/
│   │   └── style.css
│   ├── js/
│   │   └── script.js
│   └── images/
│       ├── logo.png
│       └── banner.jpg
├── README.md             # Repository documentation
└── src/                  # Source files (not served, optional)
    └── build-scripts/
```

## Setup Instructions

1. **Create a repository** in your Forgejo/Gitea instance
2. **Add a `.pages` file** in the repository root
3. **Create a `public/` folder** with your static files
4. **Push to your repository**
5. **Access your site** at `https://username.pages.example.com/repository-name/`

## File Requirements

### Required Files

- `.pages` - Configuration file to enable pages
- `public/` - Directory containing static files
- `public/index.html` - Homepage (recommended)

### Optional Files

- `public/404.html` - Custom 404 error page
- `public/robots.txt` - Search engine instructions
- `public/sitemap.xml` - Site structure for search engines
- `public/favicon.ico` - Website icon

## URL Mapping

Files in the `public/` folder are served at:

```
public/index.html       → https://username.pages.example.com/repository-name/
public/about.html       → https://username.pages.example.com/repository-name/about.html
public/css/style.css    → https://username.pages.example.com/repository-name/css/style.css
public/images/logo.png  → https://username.pages.example.com/repository-name/images/logo.png
```

## Custom Domain

To use a custom domain:

1. Edit `.pages` and add:
   ```yaml
   enabled: true
   custom_domain: www.mysite.com
   ```

2. Ensure your Traefik plugin is configured with Cloudflare credentials

3. The plugin will automatically:
   - Create DNS A record for your custom domain
   - Request SSL certificate from Let's Encrypt
   - Serve your site at the custom domain

## Profile Site

For a personal profile site (served at `https://username.pages.example.com/`):

1. Create a repository named `.profile`
2. Add `.pages` and `public/` folder as usual
3. Access at `https://username.pages.example.com/`

## Tips

- Keep files in `public/` only - other files are not served
- Use relative paths in HTML: `./css/style.css` instead of `/css/style.css`
- Optimize images for web (compress, use appropriate formats)
- Test locally before pushing
- Check Traefik logs if pages don't load

## Example index.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My Forgejo Pages Site</title>
    <link rel="stylesheet" href="./css/style.css">
</head>
<body>
    <header>
        <img src="./images/logo.png" alt="Logo">
        <h1>Welcome to My Site</h1>
    </header>

    <main>
        <p>This site is hosted on Forgejo Pages!</p>
        <a href="./about.html">About Me</a>
    </main>

    <footer>
        <p>&copy; 2025 My Name</p>
    </footer>

    <script src="./js/script.js"></script>
</body>
</html>
```

## Supported File Types

The plugin serves common static file types:

- HTML: `.html`, `.htm`
- Stylesheets: `.css`
- JavaScript: `.js`
- Images: `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.ico`
- Fonts: `.woff`, `.woff2`, `.ttf`
- Documents: `.pdf`, `.txt`, `.xml`, `.json`

Other file types are detected automatically using MIME type detection.
