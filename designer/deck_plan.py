import matplotlib.pyplot as plt
import matplotlib.patches as patches

# Deck Dimensions
width = 25.0
depth = 8.0
beam_setback = 2.0

fig, ax = plt.subplots(figsize=(12, 5))

# Draw Deck Outline
rect = patches.Rectangle((0, 0), width, depth, linewidth=2, edgecolor='brown', facecolor='none')
ax.add_patch(rect)

# Draw Ledger (House)
ax.plot([0, width], [depth, depth], color='black', linewidth=4, label="House Wall")

# Draw Beam
beam_y = depth - (depth - beam_setback) # Position from bottom
ax.plot([0, width], [beam_setback, beam_setback], color='blue', linestyle='--', linewidth=2, label="Beam")

# Draw Joists (12" O.C.)
for i in range(27):
    x = i * 1.0
    if x <= width:
        ax.plot([x, x], [0, depth], color='orange', alpha=0.3, linewidth=1)

# Draw Posts (4 posts)
posts = [1.5, 8.83, 16.16, 23.5]
for p in posts:
    circle = patches.Circle((p, beam_setback), 0.3, facecolor='black')
    ax.add_patch(circle)
    ax.text(p, beam_setback - 1.5, "POST", ha='center', fontsize=8)

ax.set_title(f"Framing Plan: {width}' x {depth}' Deck")
ax.set_aspect('equal')
plt.grid(True, alpha=0.3)
# plt.show()
plt.savefig("deck_plan.png")
print("Blueprint saved as deck_plan.png")
