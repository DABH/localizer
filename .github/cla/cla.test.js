// Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
// SPDX-License-Identifier: NCSA

// Tests for cla.js against an in-memory fake of the GitHub API. Run with: node --test .github/cla/cla.test.js

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const cla = require('./cla.js');

const repo = { owner: 'DABH', repo: 'localizer' };
const maintainer = { id: 1920464, login: 'DABH', type: 'User' };
const alice = { id: 11, login: 'alice', type: 'User' };
const bob = { id: 12, login: 'bob', type: 'User' };
const dependabot = { id: 49699333, login: 'dependabot[bot]', type: 'Bot' };

function httpError(status) {
	const err = new Error(`HTTP ${status}`);
	err.status = status;
	return err;
}

// fake returns a GitHub client backed by in-memory state: one pull request (#7), its commits, the signature
// file and the pull request's comments.
function fake({ author, commits, signatures, conflicts = 0 }) {
	const pr = { number: 7, user: author, head: { sha: 'headsha' } };
	const state = {
		statuses: [],
		comments: [],
		file: signatures && { sha: 'v1', doc: { agreement: 'CLA.md version 1', signatures } },
		writes: [],
		conflicts,
	};
	const github = {
		paginate: async (fn, params) => (await fn(params)).data,
		rest: {
			pulls: {
				listCommits: async () => ({ data: commits }),
				get: async () => ({ data: pr }),
			},
			repos: {
				getContent: async ({ path, ref }) => {
					assert.equal(path, cla.FILE);
					assert.equal(ref, cla.BRANCH);
					if (!state.file) throw httpError(404);
					const content = Buffer.from(JSON.stringify(state.file.doc)).toString('base64');
					return { data: { sha: state.file.sha, content } };
				},
				createOrUpdateFileContents: async ({ branch, sha, content, message }) => {
					assert.equal(branch, cla.BRANCH);
					if (state.conflicts > 0) {
						state.conflicts--;
						state.file = state.file || { sha: 'other', doc: { agreement: 'CLA.md version 1', signatures: [] } };
						throw httpError(409);
					}
					if (state.file && sha !== state.file.sha) throw httpError(409);
					state.writes.push(message);
					state.file = { sha: `v${state.writes.length + 1}`, doc: JSON.parse(Buffer.from(content, 'base64').toString()) };
					return { data: {} };
				},
				createCommitStatus: async (s) => {
					state.statuses.push(s);
					return { data: {} };
				},
			},
			issues: {
				listComments: async () => ({ data: state.comments }),
				createComment: async ({ body }) => {
					state.comments.push({ id: state.comments.length + 1, body, user: { login: 'github-actions[bot]', type: 'Bot' } });
					return { data: {} };
				},
				updateComment: async ({ comment_id, body }) => {
					state.comments.find((c) => c.id === comment_id).body = body;
					return { data: {} };
				},
			},
		},
	};
	return { github, state, pr };
}

const commit = (sha, author) => ({ sha, author, commit: { author: { name: author ? author.login : 'Someone' } } });
const core = { info() {} };

function opened(pr) {
	return { eventName: 'pull_request_target', repo, payload: { pull_request: pr, repository: { default_branch: 'main' } } };
}

function commented(user, body) {
	return {
		eventName: 'issue_comment',
		repo,
		payload: {
			issue: { number: 7, pull_request: {} },
			comment: { user, body, created_at: '2026-09-26T12:00:00Z', html_url: 'https://github.com/DABH/localizer/pull/7#c1' },
			repository: { default_branch: 'main' },
		},
	};
}

const lastStatus = (state) => state.statuses[state.statuses.length - 1];

test('maintainer and bot pull requests pass without a comment', async () => {
	const { github, state, pr } = fake({ author: maintainer, commits: [commit('aaaaaaa1', maintainer), commit('bbbbbbb2', dependabot)] });
	await cla.run({ github, context: opened(pr), core });
	assert.equal(lastStatus(state).state, 'success');
	assert.equal(lastStatus(state).sha, 'headsha');
	assert.equal(state.comments.length, 0);
});

test('an unsigned contributor gets a pending status and one comment', async () => {
	const { github, state, pr } = fake({ author: alice, commits: [commit('aaaaaaa1', alice), commit('bbbbbbb2', bob)] });
	await cla.run({ github, context: opened(pr), core });
	await cla.run({ github, context: opened(pr), core }); // a new push must not add a second comment
	assert.equal(lastStatus(state).state, 'pending');
	assert.equal(lastStatus(state).description, 'Waiting for 2 signatures');
	assert.match(lastStatus(state).target_url, /\/DABH\/localizer\/blob\/main\/CLA\.md$/);
	assert.equal(state.comments.length, 1);
	assert.match(state.comments[0].body, /Waiting for: @alice, @bob/);
	assert.ok(state.comments[0].body.includes(cla.SIGN_PHRASE));
});

test('signing records the signature and turns the check green', async () => {
	const { github, state, pr } = fake({ author: alice, commits: [commit('aaaaaaa1', alice)] });
	await cla.run({ github, context: opened(pr), core });
	await cla.run({ github, context: commented(alice, `  > ${cla.SIGN_PHRASE.toUpperCase().replace(/\.$/, '')}\n`), core });
	assert.deepEqual(state.file.doc.signatures, [
		{
			id: 11,
			login: 'alice',
			signed_at: '2026-09-26T12:00:00Z',
			pull_request: 7,
			comment_url: 'https://github.com/DABH/localizer/pull/7#c1',
		},
	]);
	assert.equal(lastStatus(state).state, 'success');
	assert.equal(state.comments.length, 1);
	assert.match(state.comments[0].body, /All contributors have signed/);

	// Signing again changes nothing.
	await cla.run({ github, context: commented(alice, cla.SIGN_PHRASE), core });
	assert.equal(state.writes.length, 1);
});

test('one signature is not enough when another commit author is missing', async () => {
	const { github, state, pr } = fake({ author: alice, commits: [commit('aaaaaaa1', alice), commit('bbbbbbb2', bob)], signatures: [] });
	await cla.run({ github, context: commented(alice, cla.SIGN_PHRASE), core });
	assert.equal(lastStatus(state).state, 'pending');
	assert.match(state.comments[0].body, /Waiting for: @bob$/m);
});

test('commits not linked to a GitHub account fail the check', async () => {
	const { github, state } = fake({ author: alice, commits: [commit('ccccccc3', null)], signatures: [{ id: 11, login: 'alice' }] });
	await cla.run({ github, context: commented(alice, 'recheck'), core });
	assert.equal(lastStatus(state).state, 'failure');
	assert.match(state.comments[0].body, /Commits ccccccc are authored with an email address/);
});

test('a signature written concurrently elsewhere is retried', async () => {
	const { github, state } = fake({ author: alice, commits: [commit('aaaaaaa1', alice)], conflicts: 1 });
	await cla.run({ github, context: commented(alice, cla.SIGN_PHRASE), core });
	assert.equal(state.file.doc.signatures.length, 1);
	assert.equal(lastStatus(state).state, 'success');
});

test('other comments, comments on issues and bots are ignored', async () => {
	const { github, state } = fake({ author: alice, commits: [commit('aaaaaaa1', alice)] });
	await cla.run({ github, context: commented(alice, 'I have read the agreement'), core });
	const onIssue = commented(alice, cla.SIGN_PHRASE);
	delete onIssue.payload.issue.pull_request;
	await cla.run({ github, context: onIssue, core });
	await cla.run({ github, context: commented(dependabot, cla.SIGN_PHRASE), core });
	assert.equal(state.statuses.length, 1); // only the bot's own comment triggered a check
	assert.equal(state.writes.length, 0);
});

test('a comment that merely contains the marker is not treated as the bot comment', async () => {
	const { github, state, pr } = fake({ author: alice, commits: [commit('aaaaaaa1', alice)] });
	state.comments.push({ id: 99, body: `${cla.MARKER} forged`, user: alice });
	await cla.run({ github, context: opened(pr), core });
	assert.equal(state.comments.length, 2);
	assert.equal(state.comments[0].body, `${cla.MARKER} forged`);
});

test('signature phrases are matched loosely but completely', () => {
	assert.ok(cla.isSignature(cla.SIGN_PHRASE));
	assert.ok(cla.isSignature('i have read the localizer contributor assignment agreement and i hereby sign it'));
	assert.ok(cla.isSignature(`> ${cla.SIGN_PHRASE}`));
	assert.ok(!cla.isSignature('I have read the Localizer Contributor Assignment Agreement'));
	assert.ok(!cla.isSignature(`${cla.SIGN_PHRASE} Not really.`));
	assert.ok(cla.isRecheck(' Recheck '));
});
