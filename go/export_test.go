package outside

// WorktreeRoot は test から .git を探す上限 (ceiling) を渡して 木の根の探し方を確かめる口。
// test の dir は この repo の .work/ の下に在り 上へ探すと この repo の .git に当たるため 上限で区切る。
var WorktreeRoot = worktreeRoot
