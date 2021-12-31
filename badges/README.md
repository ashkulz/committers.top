# committers.top badges

These are powered by a free [CloudFlare Workers](https://workers.cloudflare.com) plan and use the [Shields](https://shields.io) service to actually render the badge.

## Implementation

During the deployment, data is loaded from `SOURCE_URL/rank_only.json` and embedded in the final worker script (_so that external data isn't needed at runtime_).

It is expected to have the following environment variables defined:
* `CLOUDFLARE_API_TOKEN` which has [permissions to upload worker scripts](https://api.cloudflare.com/#worker-script-upload-worker)
* `CLOUDFLARE_ACCOUNT_ID` (shown on the Workers landing page in the sidebar on the right)

NOTE: two worker scripts are created (`user-badge`) and (`org-badge`) with different data embedded for both.

## One-time CloudFlare setup

1. You need to add the domain to your CloudFlare account and enable the free Workers plan.
2. Generate [an API token](https://dash.cloudflare.com/profile/api-tokens)
    * You need to specify the "Account" -> "Worker Scripts" -> "Edit" permissions
    * You need to include the account explicitly in "Account Resources"
3. Run the deployment script manually to ensure that the scripts are created
4. Configure each of the worker scripts [as described here](https://developers.cloudflare.com/workers/platform/routes#custom-routes):
    * Create an AAAA record for `100::` which is proxied by CloudFlare (orange-cloud)
    * Ensure that a route is added which maps to the worker, as no routes are added by default

NOTE: I use [DNSControl](https://stackexchange.github.io/dnscontrol/) for managing DNS, and this is the configuration I use:

```js
var REGISTRAR  = NewRegistrar('none', 'NONE');
var CLOUDFLARE = NewDnsProvider('cloudflare', 'CLOUDFLAREAPI', { manage_workers: true });

D('committers.top', REGISTRAR, DnsProvider(CLOUDFLARE),
  ALIAS('@',          'ashkulz.github.io.'),
  CNAME('www',        'ashkulz.github.io.'),
  AAAA ('user-badge', '100::', CF_PROXY_ON),
  AAAA ('org-badge',  '100::', CF_PROXY_ON),

  CF_WORKER_ROUTE("user-badge.committers.top/*", "user-badge"),
  CF_WORKER_ROUTE("org-badge.committers.top/*", "org-badge")
)

```
