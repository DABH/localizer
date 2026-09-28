---
title: Privacy Policy
description: What data Localizer's website, service and runtime libraries collect, and why.
tableOfContents: false
---

_Last updated: September 28, 2026_

This policy explains what data Localizer collects and why. Localizer is operated by **Snizyx Software
LLC**, a Wyoming limited liability company ("**we**", "**us**"). Contact us at
[saluton@locale.dev](mailto:saluton@locale.dev) for any privacy question or request.

## The runtime libraries

The Localizer runtime libraries that run inside your command-line tool collect nothing. They make no network
calls, contain no telemetry and read only the environment and operating-system settings needed to choose a
language.

## The website (locale.dev)

The website sets no cookies and uses no analytics or tracking. It is served by Amazon CloudFront, which
processes each request (including your IP address) to deliver the page; we don't keep access logs. Your browser
remembers your color theme and your Go/Python tab choice in its own local storage, which is never sent to us.

On the pricing page, the GitHub account name you type is sent from your browser directly to GitHub's public
API to look the account up.

## The hosted service

When you connect a repository with the GitHub App or the GitHub Action, we process:

- **Repository and account details:** the names and numeric IDs of the GitHub accounts and repositories
  involved, and the commit being processed.
- **Source code:** the public source code of the repository at that commit. It is downloaded into memory,
  used to find the strings your tool shows to its users, and discarded when the job ends. We don't store it.
- **Strings and translations:** the extracted strings and their translations, kept as a translation memory for
  your repositories, so the same string is never translated and paid for twice. To have it deleted, email
  us.
- **Job records and usage:** the outcome of each job (status, counts, cost), kept for 90 days, and monthly usage
  counters.
- **GitHub Action results:** the files returned to your workflow, kept encrypted for at most 7 days.
- **Installation records:** which accounts installed the GitHub App, until the App is uninstalled.
- **Logs:** technical logs, which include account and repository names but not source code, kept for 30 days.

To translate, we send the extracted strings, together with context such as the command or file each one comes
from, to Claude models made by Anthropic, running on Amazon Bedrock in AWS. Under Amazon Bedrock's
data-protection terms, these inputs and outputs aren't used to train models or shared with the model provider.

## Subscriptions

Purchases are processed by [Polar](https://polar.sh), our merchant of record, which collects your name, email
address, billing address and payment details under its own privacy policy. We never see your payment card.
Polar shares with us the details of your subscription (plan, status and dates) and the GitHub account it is
for; we can also see your name and email address in Polar's dashboard, and use them only to support your
subscription and to send you notices about the Service. Our own systems store only the GitHub account's name
and ID and the subscription's identifiers, plan, status and dates.

## Email

When you email us, we keep the correspondence as long as needed to answer you and to keep a record of it.

## Why we process this data

We process this data to provide the Service you asked for (for users in the EU and the UK: to perform our
contract with you), to keep the Service secure and prevent abuse (our legitimate interests), and to meet legal
obligations such as tax and accounting rules.

## Who else processes it

We don't sell personal data. We share it only with the providers that run parts of the Service for us:

| Provider | Role |
| --- | --- |
| Amazon Web Services | Hosting in the United States (us-east-2), the website, and translation models on Amazon Bedrock |
| GitHub | Repositories, the GitHub App and the GitHub Action |
| Polar | Payments, as merchant of record |
| Spaceship | Email forwarding for locale.dev |

We may also disclose data when the law requires us to.

## Where it is stored

Data is stored and processed in the United States. Our databases are encrypted and backed up; deleted records
can remain in backups for up to 35 days.

## Your rights

You can ask us for a copy of your personal data, and to correct or delete it. Depending on where you live, you
may have other rights, such as objecting to processing or complaining to a data-protection authority. Email
[saluton@locale.dev](mailto:saluton@locale.dev) and we'll answer within 30 days.

## Children

The Service isn't directed to children under 16, and we don't knowingly collect their data.

## Changes to this policy

We'll post changes on this page and, for material changes, notify subscribers by email before they take
effect.
