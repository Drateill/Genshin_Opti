# Deployment setup

CI/CD flow: push to `main` → GitHub Actions builds & pushes Docker images to
GHCR → workflow bumps image tags in `k8s/kustomization.yaml` and pushes that
commit → ArgoCD (watching this repo's `k8s/` path) syncs the cluster →
workflow polls the app's health endpoint to confirm the deploy worked.
Discord gets a message at every stage (build, push, manifest update, deploy
verification, and any failure).

## One-time setup

1. **Create the GitHub repo** (if not already done) at
   `github.com/Drateill/Genshin_Opti`, then push this repo to it.

2. **Discord webhook** — create a webhook on the Discord channel you want
   notifications in (Channel Settings → Integrations → Webhooks → New
   Webhook → Copy URL), then add it as a repo secret:
   `Settings → Secrets and variables → Actions → New repository secret`
   named `DISCORD_WEBHOOK_URL`.

3. **APP_HEALTH_URL repo variable** — `https://genshin.drateil.ovh`. Add it
   under `Settings → Secrets and variables → Actions → Variables` as
   `APP_HEALTH_URL`. It must be reachable from GitHub's runners, so this
   only starts working once step 4 below is done — until then, the
   `verify-deploy` job will fail/timeout even if the deploy itself is fine.

4. **Point Nginx Proxy Manager at the cluster** — `genshin.drateil.ovh` →
   the Traefik LoadBalancer on the k3s host (`192.168.1.38:80`), same
   pattern as `dh.drateil.ovh` → `darkheresy`. `k8s/ingress.yaml` already
   routes that host to the frontend Service once traffic reaches Traefik.

5. **GHCR image visibility** — after the first successful workflow run, the
   packages `genshin_opti-backend` and `genshin_opti-frontend` will exist
   under your GitHub account, defaulting to private. Either make them
   public (package settings → Change visibility), or create an
   `imagePullSecret` in the `genshin-optimizer` namespace and reference it
   from both Deployments in `k8s/`.

6. **Register the app with ArgoCD** (once, by hand, from a machine with
   access to your cluster):

   ```sh
   kubectl apply -n argocd -f k8s/argocd-application.yaml
   ```

   This is the only manifest that isn't self-managed by ArgoCD — it's what
   tells ArgoCD to start watching `k8s/` in this repo.

## Local testing

```sh
docker compose up --build
```

Backend on `:8080`, frontend on `:8081`.
