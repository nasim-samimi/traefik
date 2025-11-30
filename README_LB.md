
<p align="center">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="docs/content/assets/img/traefik.logo-dark.png">
      <source media="(prefers-color-scheme: light)" srcset="docs/content/assets/img/traefik.logo.png">
      <img alt="Traefik" title="Traefik" src="docs/content/assets/img/traefik.logo.png">
    </picture>
</p>

# Make image
export GOWORK=off
run make binary
run docker build -t nasimm/traefik:2.4.0 .
run docker push

# Use leaky bucket load-balancer

burst: integer value\
average: number of jobs per second\
period: in second, average/rate\
priority: interger value


# existing bugs
with a rate of zero the servers won't accept anything
with a burst of zero the servers accept requests :)))

the available tokens of the last server is violated if the number of requests exceeds the allowed number. Therefore, even if a server is down, 

# Use within Kubernetes environment
leakybucketLB: true