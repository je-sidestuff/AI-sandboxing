A flat, modern technical system architecture diagram rendered on a pure white (#FFFFFF) background. The diagram uses a mixed layout: an initial left-to-right horizontal section that transitions into a left-to-right horizontal chain of sequential steps. All node boxes are rectangles with slightly rounded corners (4px radius), filled with dark navy blue (#1B2A4A), and outlined with a 2px stroke of the same dark navy. All text labels inside nodes are set in a monospace font (such as Courier New or Source Code Pro), white (#FFFFFF), font size approximately 13pt, centered horizontally and vertically within each box.

The diagram is divided into two visual sections:

SECTION 1 — HEADER FLOW (left-to-right, top portion of the canvas):
Five nodes in a horizontal sequence:

Node 1 — label: 'Heuristic Input'
Node 2 — label: 'Heuristic Processor'
Node 3 — label: 'DISPATCH.json\n(type: sequence-to-new-repo)'
Node 4 — label: 'Approval Flow'
Node 5 — label: 'Create New Repo'

Node 3 sub-label '(type: sequence-to-new-repo)' has a thin amber (#FFB300) dashed underline beneath it to emphasize the dispatch type.
Between Node 4 and Node 5, add a small filled green circle (#27AE60, 10px diameter) on the connecting arrow to indicate approval granted.
Node 5 ('Create New Repo') has an additional visual treatment: a thin solid border in amber (#FFB300) instead of the default dark navy outline, 2px, to indicate repo creation as a key milestone. The node fill remains dark navy.

SECTION 2 — SEQUENTIAL STEP CHAIN (left-to-right, below and continuing from Node 5):
From Node 5, a downward-then-right connecting arrow leads into a horizontal sequential chain of step nodes. This chain proceeds left-to-right and contains the following elements in strict order:

Step Node A — label: 'Step 1:\nInitial Structure'
Timer Icon 1 — an amber/gold (#F5A623) clock/timer icon: a simple circular clock face, outlined in amber, approximately 28px in diameter, with two clock hands indicating a short interval (hands at 12 and 3 o'clock), no fill (transparent interior), 2px stroke
Step Node B — label: 'Step 2:\nBuild on Step 1'
Timer Icon 2 — identical clock/timer icon as Timer Icon 1
Step Node C — label: 'Step 3:\nBuild on Step 2'
Timer Icon 3 — identical clock/timer icon
Ellipsis Node — label: '... (N steps)' — this node uses the same dark navy box style but its border is rendered as a dashed line (4px on / 3px off) in slate gray (#708090) to indicate an indeterminate number of additional steps
Final Node — label: 'Final Repo State'

The Final Node has a solid left border accent in slate gray (#708090), 4px wide, on the left side of the node box only, to indicate the terminal output state.

Between all step nodes and ellipsis/final nodes, directional arrows in slate gray (#708090) connect right-to-left, 2px stroke, with filled triangular arrowheads. Timer icons are positioned on the arrows between step nodes, centered on the arrow line, floating above the arrow (not inside a box). Each timer icon replaces the midpoint of the arrow — draw the arrow up to 10px before the timer icon, skip the icon area, and resume the arrow 10px after the icon, so the timer appears to interrupt the flow line.

All step nodes in Section 2 are approximately 150px wide by 64px tall. The step label uses two lines: the first line 'Step N:' in 10pt monospace and the second line (the description) in 13pt monospace.

The transition from Section 1 to Section 2: draw an elbow connector from the bottom edge of Node 5 ('Create New Repo'), going downward 40px, then turning right into the beginning of the step chain. The elbow connector is slate gray (#708090), 2px stroke, with a filled arrowhead pointing right into Step Node A.

In the bottom-right corner of the image, place a small compact legend box with a 1px slate gray (#708090) border and white fill. Inside the legend, in monospace font (9pt, dark navy text), display five items:
— A short dark navy rectangle sample labeled 'Node'
— A slate gray horizontal arrow sample labeled 'Flow'
— A green dot sample labeled 'Approval'
— A small amber clock circle sample labeled 'Timed Delay'
— A short dashed slate gray rectangle outline labeled 'Indeterminate Steps'

The overall canvas is approximately 2000px wide by 560px tall, with 60px padding on all sides. Section 1 occupies roughly the top 200px of the canvas, and Section 2 occupies the lower 280px. There are no background textures, gradients, or decorative elements beyond those described. The style is minimal, clean, and professional — consistent with a software architecture reference diagram. No title text or heading should appear in the image.
