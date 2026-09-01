# IndexNow

The production website publishes the IndexNow ownership key at:

`https://flatkey.ai/acc80000-1bb5-4504-8426-b6bac35d273a.txt`

The key file is intentionally public. It proves that submitted URLs belong to
the site; it is not an application secret.

## Manual submission

From `website/`, submit one or more changed URLs:

```bash
bun run indexnow:submit -- \
  --url https://flatkey.ai/ \
  --url https://flatkey.ai/pricing
```

To submit every URL currently published in the sitemap:

```bash
bun run indexnow:submit -- \
  --sitemap https://flatkey.ai/sitemap.xml
```

The script de-duplicates URLs and sends batches of up to 10,000 URLs. The
`INDEXNOW_KEY` environment variable can override the key when rotating it; the
hosted key file name must then be updated at the same time.

## Deployment notification

The production website workflow submits the deployed sitemap URLs to IndexNow
after promoting a new Cloud Run revision. A failed notification does not fail
the deployment, so Bing outages or rate limits cannot roll back a healthy site.
