From [ROLES.md](https://github.com/knative/docs/blob/85cb853b9bd6c5b2f156a9e965634e7326694f99/community/ROLES.md#approver)

> Reviewer of the codebase for at least 3 months or 50% of project lifetime, whichever is shorter

**First review June 13 (3 months, 28 days ago)**.  [is:pr is:merged reviewed-by:markusthoemmes sort:created-asc](https://github.com/knative/serving/pulls?utf8=✓&q=is%3Apr+is%3Amerged+reviewed-by%3Amarkusthoemmes+sort%3Acreated-asc)

>  Primary reviewer for at least 10 substantial PRs to the codebase

Reviewed 5 size/M pull requests: [is:pr is:merged reviewed-by:markusthoemmes -author:markusthoemmes label:size/M](https://github.com/knative/serving/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Amerged+reviewed-by%3Amarkusthoemmes+-author%3Amarkusthoemmes+label%3Asize%2FM)

Reviewed 9 size/L pull requests: [is:pr is:merged reviewed-by:markusthoemmes -author:markusthoemmes label:size/L ](https://github.com/knative/serving/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Amerged+reviewed-by%3Amarkusthoemmes+-author%3Amarkusthoemmes+label%3Asize%2FL)

Reviewed 2 size/XXL pull requests: [is:pr is:merged reviewed-by:markusthoemmes -author:markusthoemmes label:size/XXL](https://github.com/knative/serving/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Amerged+reviewed-by%3Amarkusthoemmes+-author%3Amarkusthoemmes+label%3Asize%2FXXL)

(no size/L reviews)

**Total: 16 reviews**.

Ten substantial examples with Markus as primary reviewer:

1. [Add Revision ContainerConcurrency](https://github.com/knative/serving/pull/1917) Reviewed the whole thing and had quite a few improving comments that eventually made it in the PR
2. [Deflake breaker tests](https://github.com/knative/serving/pull/1807) Reviewed the whole thing and had quite a few improving comments that eventually made it in the PR. Proposed a change in design (see usage of the Breaker)
3. [Add a delay following inactivity before scaling to zero.](https://github.com/knative/serving/pull/1906) Proposed more tests and a few nits.
4. [Activator Header Fixes](https://github.com/knative/serving/pull/2047) Pointed out missing documentation and reviewed the whole thing.
5. [Amortize pod metrics when lameducking](https://github.com/knative/serving/pull/2109) Questioned the design, got convinced, reviewed the whole thing and proposed accepted changes
6. [Fix race condition in breaker test (#1984)](https://github.com/knative/serving/pull/2137) (Clarified some bits and lgtmed)
7. [Bump the activator to 3 replicas to test horizontal scalability.](https://github.com/knative/serving/pull/2171) (A small PR but with wide ranging effects. Identified and fixed issues not directly visible in the code diff)
8. [Avoid emitting headers that later reject requests](https://github.com/knative/serving/pull/2240) (Discussed about the implementation, suggested improvements)
9. [Provide an e2e test that asserts autoscaler stability](https://github.com/knative/serving/pull/2345) (Discussion made the test richer and more stable)
10. [try to start shutdown without always waiting for a specific time](https://github.com/knative/serving/pull/2365) (Iut I dug very deep into this to lead the contributor on the right path)
11. [Disable scale-to-zero during activation (#2155)](https://github.com/knative/serving/pull/2386) (I discussed a change in the design of the PR, which ultimately resulted in a better quality)

> Reviewed or merged at least 30 PRs to the codebase


Merged 29 pull requests: [is:pr is:merged author:markusthoemmes ](https://github.com/knative/serving/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Amerged+author%3Amarkusthoemmes)

Reviewed 16 pull requests: [is:pr is:merged reviewed-by:markusthoemmes -author:markusthoemmes ](https://github.com/knative/serving/pulls?utf8=%E2%9C%93&q=is%3Apr+is%3Amerged+reviewed-by%3Amarkusthoemmes+-author%3Amarkusthoemmes)

**Total: 45 merges or reviews**.

> Nominated by an area lead

Nominated by @josephburnett, the author of this pull request.
