// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Checks that everyone who contributed to a pull request has signed the Contributor Assignment Agreement
// (CLA.md), and records signatures. Run by .github/workflows/cla.yml through actions/github-script.
// Signatures are kept as JSON on the cla-signatures branch, so every one of them is in the git history.

'use strict';

const VERSION = 1;
const BRANCH = 'cla-signatures';
const FILE = `signatures/v${VERSION}.json`;
const STATUS_CONTEXT = 'CLA';
const MARKER = '<!-- localizer-cla -->';
const SIGN_PHRASE = 'I have read the Localizer Contributor Assignment Agreement and I hereby sign it.';
// Maintainers, whose work already belongs to Snizyx Software LLC (lowercase GitHub logins).
const ALLOWLIST = new Set(['dabh']);

const normalize = (s) =>
	(s || '')
		.trim()
		.replace(/^>\s*/, '') // pasted from the quote in the bot's comment
		.replace(/\s+/g, ' ')
		.replace(/[.!]+$/, '')
		.toLowerCase();
const isSignature = (body) => normalize(body) === normalize(SIGN_PHRASE);
const isRecheck = (body) => normalize(body) === 'recheck';

const agreementURL = (repo, branch) => `https://github.com/${repo.owner}/${repo.repo}/blob/${branch}/CLA.md`;

// contributors lists who must sign for a pull request: its author and the author of every commit.
// Commits whose author email isn't linked to a GitHub account can't be matched to a signature; they are
// returned separately.
async function contributors(github, repo, pr) {
	const commits = await github.paginate(github.rest.pulls.listCommits, {
		...repo,
		pull_number: pr.number,
		per_page: 100,
	});
	const users = new Map();
	const add = (u) => {
		if (u && u.type !== 'Bot' && !ALLOWLIST.has(u.login.toLowerCase())) users.set(u.id, u.login);
	};
	add(pr.user);
	const unlinked = [];
	for (const c of commits) {
		if (c.author) add(c.author);
		else unlinked.push(c.sha.slice(0, 7));
	}
	return { users, unlinked };
}

async function loadSignatures(github, repo) {
	try {
		const { data } = await github.rest.repos.getContent({ ...repo, path: FILE, ref: BRANCH });
		return { sha: data.sha, doc: JSON.parse(Buffer.from(data.content, 'base64').toString('utf8')) };
	} catch (err) {
		if (err.status !== 404) throw err;
		return { sha: undefined, doc: { agreement: `CLA.md version ${VERSION}`, signatures: [] } };
	}
}

// findComment returns this workflow's comment on a pull request, if it has posted one.
async function findComment(github, repo, number) {
	const comments = await github.paginate(github.rest.issues.listComments, {
		...repo,
		issue_number: number,
		per_page: 100,
	});
	return comments.find((c) => c.user && c.user.type === 'Bot' && (c.body || '').includes(MARKER));
}

function pendingComment(url, missing, unlinked) {
	const lines = [
		MARKER,
		'Thank you for your contribution! Before it can be merged, everyone who contributed to this pull request',
		`needs to sign the [Localizer Contributor Assignment Agreement](${url}). It assigns the copyright in`,
		'your contribution to Snizyx Software LLC and gives you back a license to use it however you like.',
		'',
		'To sign, post this comment:',
		'',
		`> ${SIGN_PHRASE}`,
		'',
	];
	if (missing.length) {
		lines.push(`Waiting for: ${missing.map((login) => `@${login}`).join(', ')}`, '');
	}
	if (unlinked.length) {
		lines.push(
			`Commits ${unlinked.join(', ')} are authored with an email address that isn't linked to a GitHub`,
			"account, so they can't be matched to a signature. Link the address to your account, or rewrite the",
			'commits with an address that is linked, then comment `recheck`.',
			'',
		);
	}
	lines.push("You only need to sign once. If this check doesn't update, comment `recheck`.");
	return lines.join('\n');
}

const SIGNED_COMMENT = `${MARKER}\nAll contributors have signed the Contributor Assignment Agreement. Thank you!`;

// check sets the CLA commit status on the pull request's head commit and keeps one comment up to date.
async function check({ github, repo, core }, pr, defaultBranch) {
	const { users, unlinked } = await contributors(github, repo, pr);
	const { doc } = await loadSignatures(github, repo);
	const signed = new Set(doc.signatures.map((s) => s.id));
	const missing = [...users].filter(([id]) => !signed.has(id)).map(([, login]) => login);
	const ok = missing.length === 0 && unlinked.length === 0;
	const url = agreementURL(repo, defaultBranch);

	let state = 'success';
	let description = 'All contributors have signed the agreement';
	if (missing.length) {
		state = 'pending';
		description = `Waiting for ${missing.length} signature${missing.length === 1 ? '' : 's'}`;
	} else if (unlinked.length) {
		state = 'failure';
		description = "Some commits aren't linked to a GitHub account";
	}
	await github.rest.repos.createCommitStatus({
		...repo,
		sha: pr.head.sha,
		context: STATUS_CONTEXT,
		state,
		description,
		target_url: url,
	});

	const existing = await findComment(github, repo, pr.number);
	const body = ok ? SIGNED_COMMENT : pendingComment(url, missing, unlinked);
	if (existing) {
		if (existing.body !== body) await github.rest.issues.updateComment({ ...repo, comment_id: existing.id, body });
	} else if (!ok) {
		await github.rest.issues.createComment({ ...repo, issue_number: pr.number, body });
	}
	core.info(`CLA #${pr.number}: ${description}${missing.length ? ` (${missing.join(', ')})` : ''}`);
	return { ok, missing, unlinked };
}

// sign records the signature of a comment's author. Concurrent signatures on other pull requests make the
// update fail with a conflict; it is then retried on the new version of the file.
async function sign({ github, repo, core }, comment, pr) {
	const user = comment.user;
	if (!user || user.type === 'Bot') return;
	for (let attempt = 0; attempt < 5; attempt++) {
		const { sha, doc } = await loadSignatures(github, repo);
		if (doc.signatures.some((s) => s.id === user.id)) {
			core.info(`CLA: @${user.login} has already signed`);
			return;
		}
		doc.signatures.push({
			id: user.id,
			login: user.login,
			signed_at: comment.created_at,
			pull_request: pr.number,
			comment_url: comment.html_url,
		});
		try {
			await github.rest.repos.createOrUpdateFileContents({
				...repo,
				branch: BRANCH,
				path: FILE,
				sha,
				message: `Record the signature of @${user.login} (#${pr.number})`,
				content: Buffer.from(`${JSON.stringify(doc, null, 2)}\n`).toString('base64'),
			});
			core.info(`CLA: recorded the signature of @${user.login}`);
			return;
		} catch (err) {
			if (err.status !== 409 && err.status !== 422) throw err;
		}
	}
	throw new Error(`CLA: could not record the signature of @${user.login}`);
}

async function run({ github, context, core }) {
	const repo = context.repo;
	const defaultBranch = context.payload.repository.default_branch;
	const ctx = { github, repo, core };
	if (context.eventName === 'pull_request_target') {
		return check(ctx, context.payload.pull_request, defaultBranch);
	}
	if (context.eventName === 'issue_comment') {
		const { issue, comment } = context.payload;
		if (!issue.pull_request) return;
		const signing = isSignature(comment.body);
		if (!signing && !isRecheck(comment.body)) return;
		const { data: pr } = await github.rest.pulls.get({ ...repo, pull_number: issue.number });
		if (signing) await sign(ctx, comment, pr);
		return check(ctx, pr, defaultBranch);
	}
}

module.exports = { run, isSignature, isRecheck, SIGN_PHRASE, BRANCH, FILE, MARKER };
