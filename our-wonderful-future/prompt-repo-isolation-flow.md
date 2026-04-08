A flat, modern technical system architecture diagram rendered on a pure white (#FFFFFF) background. The diagram uses an orthogonal (left-to-right) layout with clean, sharp geometry and no drop shadows. All node boxes are rectangles with slightly rounded corners (4px radius), filled with dark navy blue (#1B2A4A), and outlined with a 2px stroke of the same dark navy. All text labels inside nodes are set in a monospace font (such as Courier New or Source Code Pro), white (#FFFFFF), font size approximately 13pt, centered horizontally and vertically within each box.

The diagram contains exactly nine labeled nodes connected in a strict left-to-right sequence by directional arrows. The nodes are, in order:

Node 1 — label: 'Heuristic Input'
Node 2 — label: 'Heuristic Processor'
Node 3 — label: 'DISPATCH.json\n(type: repo-isolation)'
Node 4 — label: 'Approval Flow'
Node 5 — label: 'Isolated Clone\n(temp workspace)'
Node 6 — label: 'Agent Makes\nChanges'
Node 7 — label: 'Push Feature\nBranch'
Node 8 — label: 'Pull Request\nOpened'
Node 9 — label: 'Target Repo\n(awaiting review)'

Each node box is approximately 160px wide by 60px tall, with consistent sizing across all nodes. Multi-line labels use a line break between the primary label and the parenthetical or descriptive sub-label, with the sub-label in a slightly smaller size (10pt).

Between each consecutive pair of nodes, draw a single directional arrow pointing right. Arrows are drawn as straight horizontal lines in slate gray (#708090) with a solid filled triangular arrowhead (10px) at the destination end. Arrow lines are 2px in stroke weight. The arrows connect the right edge of one node box to the left edge of the next, with 20px of horizontal gap on each side.

Node 3 ('DISPATCH.json (type: repo-isolation)') should have a thin amber (#FFB300) dashed underline beneath the sub-label text '(type: repo-isolation)' to emphasize the dispatch type.

Between Node 4 ('Approval Flow') and Node 5, add a small filled green circle (#27AE60, 10px diameter) on the connecting arrow to indicate approval granted.

Nodes 5 through 8 — that is, 'Isolated Clone (temp workspace)', 'Agent Makes Changes', 'Push Feature Branch', and 'Pull Request Opened' — are enclosed together inside a prominent isolation zone. This isolation zone is rendered as a rectangle that contains all four nodes with 24px of padding around them on all sides. The isolation zone border is a dashed line, dash pattern 8px on / 4px off, in a vivid orange color (#E8650A), 2.5px stroke weight. The zone's fill is an extremely subtle warm tint (#FFF8F4) — nearly white but faintly warm — so the nodes inside are still clearly legible against their dark navy fill. Inside the isolation zone, in the top-left corner, place a small text label in monospace font, 9pt, orange (#E8650A), reading 'ISOLATION BOUNDARY'. Do not add any other decorative elements inside the zone.

Node 9 ('Target Repo (awaiting review)') is outside the isolation zone, to the right, connected by the usual slate gray directional arrow. This node has a solid left border accent, 4px wide, in slate gray (#708090), on the left edge of the box only.

In the bottom-right corner of the image, place a small compact legend box with a 1px slate gray (#708090) border and white fill. Inside the legend, in monospace font (9pt, dark navy text), display four items:
— A short dark navy rectangle sample labeled 'Node'
— A slate gray horizontal arrow sample labeled 'Flow'
— A green dot sample labeled 'Approval'
— A short dashed orange line sample labeled 'Isolation Boundary'

The overall diagram should be horizontally centered in a canvas approximately 1800px wide by 420px tall, with 60px of padding on all sides. There are no background textures, gradients, or decorative elements beyond those described. The style is minimal, clean, and professional — consistent with a software architecture reference diagram. No title text or heading should appear in the image.
