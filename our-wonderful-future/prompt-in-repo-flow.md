A flat, modern technical system architecture diagram rendered on a pure white (#FFFFFF) background. The diagram uses an orthogonal (left-to-right) layout with clean, sharp geometry and no drop shadows. All node boxes are rectangles with slightly rounded corners (4px radius), filled with dark navy blue (#1B2A4A), and outlined with a 2px stroke of the same dark navy. All text labels inside nodes are set in a monospace font (such as Courier New or Source Code Pro), white (#FFFFFF), font size approximately 13pt, centered horizontally and vertically within each box.

The diagram contains exactly six labeled nodes connected in a strict left-to-right sequence by directional arrows. The nodes are, in order:

Node 1 — label: 'Heuristic Input'
Node 2 — label: 'Heuristic Processor'
Node 3 — label: 'DISPATCH.json\n(type: in-repo)'
Node 4 — label: 'Approval Flow'
Node 5 — label: 'Agent Checkout\n(target branch)'
Node 6 — label: 'Direct Commit\nto Repo'

Each node box is approximately 160px wide by 60px tall, with consistent sizing across all nodes. Multi-line labels use a line break between the primary label and any parenthetical sub-label, with the sub-label in a slightly smaller size (10pt) and a lighter weight if possible.

Between each consecutive pair of nodes, draw a single directional arrow pointing right. Arrows are drawn as straight horizontal lines in slate gray (#708090) with a solid arrowhead (filled triangle, 10px) at the destination end. Arrow lines are 2px in stroke weight. The arrows connect the right edge of one node box to the left edge of the next node box, with 20px of horizontal gap on each side between the arrowhead/tail and the node border.

Node 3 ('DISPATCH.json (type: in-repo)') should have an additional visual treatment: a thin amber (#FFB300) dashed underline beneath the sub-label text '(type: in-repo)' to emphasize the dispatch type, but the node fill and border remain dark navy.

Between Node 4 ('Approval Flow') and Node 5 ('Agent Checkout (target branch)'), add a small visual indicator of a checkmark or green dot (filled circle, #27AE60, 10px diameter) on the arrow to indicate approval granted.

Node 6 ('Direct Commit to Repo') should have a subtle solid left border accent in slate gray (#708090), 4px wide, on the left side of the node box only, to draw attention to the final output step.

Critically, there is NO pull request node, NO isolated workspace zone, and NO branching. The flow is strictly linear and direct — this absence of a PR/isolation layer is a key semantic feature of the diagram. Do not add any fork/branch symbols.

In the bottom-right corner of the image, place a small, compact legend box with a thin 1px slate gray (#708090) border and white fill. Inside the legend, display three items in monospace font (9pt, dark navy text):
— A short dark navy rectangle sample labeled 'Node'
— A slate gray horizontal arrow sample labeled 'Flow'
— A green dot sample labeled 'Approval'

The overall diagram should be horizontally centered in a canvas approximately 1400px wide by 400px tall, with 60px of padding on all sides. There are no background textures, gradients, or decorative elements. The style is minimal, clean, and professional — consistent with a software architecture reference diagram. No title text or heading should appear in the image.
