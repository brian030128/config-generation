FROM node:23-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install
COPY . .
RUN npm run build

# NOTE: This image runs as root because nginx must bind to privileged port 80
# in the production K8s deployment (see config-generation-gitops:
# charts/config-gen/templates/frontend/configmap.yaml `listen 80;`).
# Non-root hardening is enforced at the platform layer instead:
#   - NetworkPolicy restricts ingress to the ingress-nginx controller
#   - K8s SecurityContext / PodSecurityStandards can drop capabilities
# Switching to nginxinc/nginx-unprivileged would require coordinated port
# changes across ConfigMap / Deployment / Service / NetworkPolicy.
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
