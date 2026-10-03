---
title: Enterprise SLA and support
description: Localizer's uptime commitment, service credits and support response times for Enterprise plans.
tableOfContents: false
editUrl: false
---

_Last updated: October 2, 2026_

This service level agreement (**SLA**) applies to Enterprise subscriptions to the hosted Localizer service,
and to Custom agreements that refer to it. It is part of the [Terms of Service](/terms/).

## Uptime commitment

The Service will be available at least **99.9%** of each calendar month.

- The **Service** means the Localizer API at `api.locale.dev`, which the GitHub Action and the GitHub App
  use, and the processing of sync jobs.
- A minute is **unavailable** if, throughout that minute, the API answers valid requests only with server
  errors or doesn't answer at all, or if sync jobs the Service has accepted don't start within 15 minutes.
- **Monthly uptime** is the share of minutes in the month that weren't unavailable.

## What isn't covered

Unavailability doesn't count toward the commitment when it's caused by:

- GitHub, including GitHub outages and GitHub's API limits;
- your repository or configuration, for example an uninstalled GitHub App, a broken workflow, or a used-up
  monthly allowance;
- scheduled maintenance announced at least 48 hours in advance, for at most 4 hours a month;
- events beyond our reasonable control;
- a suspension under the [Terms of Service](/terms/).

The quality of individual translations isn't covered by this SLA.

## Service credits

If monthly uptime falls short of the commitment, you're entitled to a credit on that month's Enterprise fee:

| Monthly uptime | Credit |
| --- | --- |
| Below 99.9% | 10% |
| Below 99.0% | 25% |

To claim a credit, email [saluton@locale.dev](mailto:saluton@locale.dev) within 30 days after the end of the
month, with the times and the repositories affected. We refund credits through Polar, or apply them to your next
invoice under a Custom agreement. Credits are capped at 25% of the month's fee and are the sole remedy for
unavailability.

## Support

Email [saluton@locale.dev](mailto:saluton@locale.dev) from the address on your subscription. Enterprise
requests get a first response within **one business day** (Monday to Friday, excluding US federal holidays).
Requests on other plans are answered as soon as we can.

## Security reviews

For Enterprise and Custom customers, we complete your security questionnaire on request. A data processing
agreement, with the EU Standard Contractual Clauses and the UK Addendum, is available to any paid plan on
request. Our security model is described in [Security](/security/).

## Changes

We may update this SLA. Changes never reduce the commitments that apply during a period you have already paid
for.
