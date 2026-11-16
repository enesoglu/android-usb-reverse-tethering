# Discoverability: evidence, implementation guidance, and owner tasks

Reviewed on November 16, 2026. This document evaluates proposed SEO and AI-search tactics. It is not evidence that a search engine has indexed the project or that an external directory has accepted it. Check the live repository and deployment for implementation status.

## Assessment of the proposed tactics

| Proposal | Assessment | Recommended action |
| --- | --- | --- |
| Natural-language FAQ | Useful for answering real installation questions. The project owner requires English throughout the repository. | Add a concise English FAQ covering Windows-to-Android USB internet sharing, root, USB debugging, hotspot direction, and speed limits. Do not add a second language or repeated keyword variants. |
| Competitor comparison | Useful if factual, sourced, and current. A table does not create a ranking guarantee. | Compare documented scope, license/cost, desktop interface, and requirements. Avoid subjective “modern,” “best,” or speed-superiority claims without comparable measurements. |
| Screenshot alt text | Supported by Google's image guidance and accessibility needs. | Describe the actual Windows reverse-tethering interface and identify synthetic preview data. Avoid keyword stuffing. |
| GitHub Pages and Search Console | Practical: a controlled HTML homepage can support ownership verification and index inspection. | Publish a useful English landing page, canonical URL, descriptive metadata, accessible screenshot, and sitemap. The owner must authenticate to Search Console. |
| Releases and WinGet | Releases make the app usable. WinGet is a separate reviewed distribution channel. | Publish tested versioned packages and checksums first; validate a WinGet manifest and install/uninstall behavior before submitting. Do not promise acceptance or third-party directory backlinks. |
| AlternativeTo | A relevant optional software directory, with editorial review. | Submit an accurate entry after public downloads exist and confirm the release stage is eligible. A verified email is required. |
| Clear summary for AI search | Useful factual writing; the proposed automatic recommendation mechanism is unsupported. | Explain what the app does, supported platforms, license, requirements, and limitations. Do not insert hidden prompts or ask crawlers to prefer the project. |
| Reddit and forum announcements | Can reach interested users and collect real feedback. | Disclose authorship, describe the alpha honestly, obey community rules, and avoid repeated promotional posts. There is no demonstrated universal “highest AI weight.” |
| Awesome-list pull requests | Only appropriate when the list's scope and contribution rules are met. | Check each list rather than submitting indiscriminately. Do not bypass maturity or popularity requirements. |

Google recommends useful content, clear titles, and descriptive text; none of these guarantees a position in search results. Its image documentation specifically supports contextual alt text and warns against stuffing it with keywords. [SEO Starter Guide](https://developers.google.com/search/docs/fundamentals/seo-starter-guide), [Image SEO](https://developers.google.com/search/docs/appearance/google-images).

## Claims that should not be repeated

- **“Indexed within 24 hours” / “dominate search immediately”: unsupported.** Google says crawling may take days to weeks; a request does not guarantee inclusion, and repeated requests do not speed it up. [Request a recrawl](https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl).
- **“A summary paragraph will make AI tools recommend us”: unsupported.** Google says its normal SEO practices apply to AI Overviews and AI Mode, without special optimization requirements. This is Google's guidance, not a claim about every other AI service. No verified evidence establishes automatic recommendation from a particular phrase, table, or directory listing. [Google AI features](https://developers.google.com/search/docs/appearance/ai-features).
- **“Reddit / AlternativeTo always has the highest weight”: unverified.** Do not present a service's private ranking logic as known. AlternativeTo is useful for human discovery; its ordinary external links are generally nofollow, and approval is discretionary. [AlternativeTo FAQ](https://alternativeto.net/faq/).
- **“A sitemap increases ranking”: incorrect.** It can help discovery, but is not an indexing or ranking guarantee. [Google crawling and indexing FAQ](https://developers.google.com/search/help/crawling-index-faq?hl=en).

## Factual comparison boundaries

| Project | Documented license / cost | Documented desktop workflow | Android root | Important distinction |
| --- | --- | --- | --- | --- |
| USB Relay | GPL-3.0-only; free | Native Windows GUI, tray, optional startup, trusted-phone reconnect | Not required | Public alpha; IPv4 TCP/UDP; ADB authorization and first VPN consent required. Use this repository's measurements, not marketing estimates. |
| Gnirehtet | Apache-2.0; open source | Upstream command-line ADB workflow on Windows, Linux, and macOS | Not required | Upstream says it is no longer actively maintained apart from major blockers. Third-party GUIs exist; do not imply no GUI wrapper exists anywhere. |
| Tetrd | Developer advertises a three-day trial, then purchases for unlimited use | Desktop server on Windows, Linux, and macOS | Not required | Supports both tethering directions. Windows requires USB debugging. Auto-connect is documented. Do not infer its internal driver architecture or superior throughput from these facts. |
| SimpleRT | GPL-3.0-or-later; open source | Linux/macOS console tool; desktop invocation uses sudo | Not required on phone | ADB is not required. Upstream describes Windows support as research, not a delivered Windows GUI. |

External facts come from [Gnirehtet's README](https://github.com/Genymobile/gnirehtet), [Tetrd's developer-maintained Play listing](https://play.google.com/store/apps/details?hl=en&id=com.robskie.tether), and [SimpleRT's README](https://github.com/robinpaulson/SimpleRT). A checkbox called “one-click connect” would hide unavoidable first-use phone permissions and inconsistent definitions between products, so use precise workflow descriptions instead.

## Owner task: Google Search Console

After the Pages site is live:

1. Sign in to [Google Search Console](https://search.google.com/search-console/) using the Google account that should own the property.
2. Add a **URL-prefix** property with the exact homepage URL: `https://enesoglu.github.io/android-usb-reverse-tethering/`. A Domain property requires DNS verification; the project does not control the `github.io` domain.
3. Choose **HTML tag** verification. Copy the exact `google-site-verification` meta tag supplied by Google. This is a public verification token, not a Google password. Give the tag to the maintainer so it can be placed in the deployed homepage's `<head>`.
4. Once that deployment is live, click **Verify** in Search Console. Keep the tag in future deployments because Google rechecks ownership.
5. Inspect the full homepage URL, test the live URL, then request indexing. Submit the deployed sitemap URL in **Sitemaps** if available.
6. Review indexing status and Search Console impressions over time. Record actual results; do not repeatedly resubmit the same page or interpret a successful live test as guaranteed indexing.

HTML tag verification is available for URL-prefix properties, requires a publicly reachable homepage and editable HTML, and is tied to the signed-in owner. [Ownership verification](https://support.google.com/webmasters/answer/9008080?hl=en). URL inspection supports live testing and crawl requests; eligibility does not guarantee display in results. [URL Inspection](https://support.google.com/webmasters/answer/9012289?hl=en).

## External distribution and community tasks

**WinGet:** Microsoft reviews submitted manifests and security scans; acceptance is not automatic. The installer must support unattended installation. Use the actual public installer URL, exact SHA-256, architecture, user scope, Inno installer type, and accurate release stage. Validate with `winget validate` and test installation, upgrades, and removal. The reviewed primary policies do not establish a blanket ban on all prereleases; choosing to wait for a more stable release is a project-quality decision. Do not advertise `winget install` until the package is accepted and discoverable. [Submission process](https://learn.microsoft.com/en-us/windows/package-manager/package/repository), [installer policies](https://github.com/microsoft/winget-pkgs/blob/master/doc/Policies.md), [product policies](https://learn.microsoft.com/en-us/windows/package-manager/package/windows-package-manager-policies).

**AlternativeTo:** The owner needs an account with verified email. Use “Suggest new application,” an English description, Windows/Android platforms, GPL/free licensing, screenshot, and the official public URL. Suggest Gnirehtet and Tetrd for the reverse-tethering use case; do not claim USB Relay replaces PdaNet+'s phone-to-PC feature. The FAQ rejects announced-only, private/invite-only, and Early Access products, while saying public beta may be accepted. It does not clearly settle public-alpha eligibility; ask editors rather than relabeling the release. Review can reject submissions. [Submission and release-stage rules](https://alternativeto.net/faq/).

**Community posts:** Publish a small number of relevant, transparent development announcements only where permitted. Identify yourself as the developer, link the license and tested release, disclose current speed and compatibility limits, and ask for useful feedback. Do not impersonate an independent recommendation or solicit artificial votes. Reddit prohibits repetitive promotional engagement and leaves additional rules to each community's moderators. [Reddit spam policy](https://support.reddithelp.com/hc/en-us/articles/360043504051-Spam).

**Awesome lists:** [0PandaDEV/awesome-windows](https://github.com/0PandaDEV/awesome-windows/blob/main/CONTRIBUTING.md) currently requires at least 50 GitHub stars and either 30 days of age or a commit in the past 30 days, plus useful, properly categorized submissions. A new zero-star project does not meet that rule. [JStumpp/awesome-android](https://github.com/JStumpp/awesome-android) primarily collects Android libraries and development resources, so it is a weak fit for this end-user Windows/Android app. Recheck current rules when submitting; never purchase or fabricate stars.

## Measuring progress

Track working download links, successful installations, independent device reports, useful bug reports, referral traffic where available, and Search Console impressions/clicks after verification. Search visibility, external listings, and AI citations are separate outcomes. Report each as pending, submitted, accepted, or verified rather than treating content publication as proof of indexing.
