function diff_color_add() return DIFF_COLOR_ADD end
function diff_color_remove() return DIFF_COLOR_REMOVE end
function diff_color_change() return DIFF_COLOR_CHANGE end

-- Add minimal runtime tests for missing input, self-diff, and left/right diff
if not content then
  error('Missing input file')
end
if not left or not right then
  error('Missing left or right file')
end
if left == right then
  error('Self-diff is not supported')
end
