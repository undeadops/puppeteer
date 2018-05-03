# Puppeteer

AutoScaling and Restarting Deis Workflow Applications based on Thresholds in RabbitMQ Queues.

First iteration, and I'm sure it needs refactoring as well as some stability handling improvements.

If looking for Pod AutoScaling, the https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/ might be more of what your looking for.  This was a scratch my own itch project. A proof of concept for doing more things within the Kubernetes API(not that this uses it), but utilizing Kubernetes as a framework for making hard stuff easy. 

## Basic Premise

Monitor RabbitMQ Queue, if it reaches the Threshold, scale worker pods by ScaleBy count.  If Queue, is under watermark, scale number of pods down by 1(not currently configurable).  Idea being to slowly scale back resources allocated, so we potentially can remove hosts from the kubernetes cluster at night(different process for that).

## Configuration

Configuration for monitored queues is read from the puppeteer.yml file in the root of this repo.  Information for connecting to rabbitmq and deis is stored in environment variables.

Designed to run in cluster, with a few Environment variables passed in.

    DEIS_USERNAME: Username for Deis user, used to restart/scale processes
    DEIS_PASSWORD: Password For user
    DEIS_URL:      Deis URL for Controller
    RABBITMQ_URL:  URL Name in puppeteer.yml for RabbitMQ Admin eg: http://user:pass@rabbitmq:15762
    STATE:         Enabled - Has to be set to enable scaling otherwise monitor mode.


### Known Deficiencies

- Logs a bit too much
- Documentation needs work
- Doesn't handle Timeout/unresponsive RabbitMQ API or Deis API issues.
- Need Metrics/Prometheus Endpoint added... shouldn't be to difficult to do.
- With alot of Queues, it will make alot of calls to RabbitMQ, could change to make one API call and store/filter for each process looking up queue counts.
