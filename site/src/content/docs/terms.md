---
title: Terms of Service
description: The terms for using Localizer's hosted service.
tableOfContents: false
editUrl: false
---

_Last updated: October 2, 2026_

These terms govern the hosted Localizer service (the **Service**): the Localizer GitHub App, the API at
`api.locale.dev` that the Localizer GitHub Action calls, and the translations they produce. The Service is
operated by **Snizyx Software LLC**, a Wyoming limited liability company ("**we**", "**us**"). You can reach
us at [saluton@locale.dev](mailto:saluton@locale.dev).

The Localizer runtime libraries (the Go module `github.com/DABH/localizer` and the Python package
`localizer`) are open source under the University of Illinois/NCSA license. That license, not these terms,
governs them.

## 1. Agreement

By installing the GitHub App, running the GitHub Action against the Service, or subscribing to a plan, you
agree to these terms. If you do so for an organization, you confirm that you may bind it to these terms, and
"you" means that organization.

## 2. The Service

The Service reads the source code of the public GitHub repositories you connect, extracts the strings your
command-line tool shows to its users, translates them with AI models, and proposes the translations as pull
requests, or as files returned to your workflow when you use the GitHub Action. The Service works with
public repositories only.

We may improve and change the Service. We won't materially reduce the features of a paid plan during a period
you have already paid for.

## 3. Plans and eligibility

A subscription is for one GitHub account (a personal account or an organization) and covers the public
repositories that account owns. The plans are described on the [pricing page](/pricing/).

- **Solo** may only be bought for the personal GitHub account of an individual, and only covers
  repositories that account owns.
- **Team** may be bought for a personal account, or for an organization that, together with its affiliates,
  has fewer than 100 employees.
- **Enterprise** may be bought for a personal account or for an organization of any size, and includes the
  commitments in the [SLA](/sla/).
- **Custom** agreements are made in writing with us; where they differ from these terms, the written
  agreement applies.

If you no longer qualify for your plan, for example because your organization has grown to 100 employees or
more, you must move to a plan you qualify for (Enterprise or Custom) by your next renewal.

Each plan includes a number of translations per month. One translation is one string translated into one
language. Allowances reset on the first day of each calendar month (UTC) and unused translations don't roll
over. When an allowance runs out, the Service translates what fits and continues after the allowance renews.

We may also give accounts access without a subscription, at our discretion.

## 4. Payment

Subscriptions are sold through [Polar](https://polar.sh), which acts as our merchant of record: Polar processes
your payment, charges any applicable sales tax or VAT, issues your receipts, and provides the customer
portal where you can change or cancel your plan. Polar's own terms apply to the purchase.

Subscriptions renew every month until canceled. A cancellation takes effect at the end of the period you have
paid for, and the Service keeps working until then. Payments are not refundable for partial periods, except
where the law requires otherwise or under the [SLA](/sla/). If you cancel within 14 days of your first
purchase, email us and we will refund that charge through Polar. Statutory withdrawal rights of consumers in
the EU and UK are not affected.

We may change prices by giving you at least 30 days' notice. A new price applies from your first renewal
after the notice period.

## 5. Your repositories and your translations

You keep all rights to your code and content. You allow us to access, copy and process the repositories you
connect, and the strings and translations the Service produces for them, only as needed to provide the
Service. This includes keeping the strings and their translations as a translation memory for that
repository, so that the same string is never translated and paid for twice within it.

The translations the Service produces for your repositories are yours. We claim no rights in them and assign to
you any rights we may have in them.

You are responsible for having the right to connect each repository you connect.

## 6. AI translations

Translations are produced by AI models and can contain mistakes. The Service checks every translation for
intact placeholders, commands and similar technical details, but it doesn't guarantee that a translation is
accurate or suitable for your purpose. Review the pull requests before merging them.

## 7. Acceptable use

You may not:

- connect repositories you aren't authorized to connect;
- get around plan limits, eligibility rules or the one-account scope of a subscription, for example by moving
  repositories between accounts to share a subscription;
- interfere with or disrupt the Service, or probe it for vulnerabilities outside our
  [security policy](https://github.com/DABH/localizer/security/policy);
- use the Service to process anything other than your software's own user-facing strings, or content that is
  unlawful;
- resell the Service.

We may suspend the Service for an account that breaks these rules. Where we can, we'll tell you first and give
you a chance to fix the problem.

## 8. Ending the agreement

You can stop using the Service at any time: cancel your subscription in Polar's customer portal, and uninstall
the GitHub App or remove the GitHub Action. We may end or suspend your access if you break these terms or don't
pay. We may also discontinue the Service by giving at least 30 days' notice, in which case we refund the unused
part of any period you have paid for.

Pull requests the Service has already opened stay in your repositories. What happens to the data we hold is
described in the [Privacy Policy](/privacy/).

## 9. Disclaimer

Except for the commitments in the [SLA](/sla/) that apply to Enterprise plans, the Service is provided "as is"
and "as available", without warranties of any kind, to the fullest extent the law allows.

## 10. Limitation of liability

To the fullest extent the law allows, neither you nor we are liable for indirect, incidental, special,
consequential or punitive damages, or for lost profits, revenue or data. Our total liability for all claims
relating to the Service is limited to the amount you paid for the Service in the 12 months before the event
that gave rise to the claim.

## 11. Changes to these terms

We may update these terms. We'll announce material changes on this page and by email at least 30 days before
they take effect. If you keep using the Service after that, the updated terms apply.

## 12. Governing law

These terms are governed by the laws of the State of Wyoming, United States, and disputes are subject to the
courts located in Wyoming, except where the law of your place of residence gives you rights that can't be
waived.

## 13. Contact

Snizyx Software LLC · [saluton@locale.dev](mailto:saluton@locale.dev)
