function diff_color_add() return DIFF_COLOR_ADD end
function diff_color_remove() return DIFF_COLOR_REMOVE end
function diff_color_change() return DIFF_COLOR_CHANGE end

-- Fix line counting by iterating over gmatch results
local lines = {}
for line in content:gmatch('([^
]+)') do
  table.insert(lines, line)
end
