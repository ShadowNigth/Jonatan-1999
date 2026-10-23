-- 🦊 Kitsune Go Wayland — Neovim Go Studio (Sakura Catppuccin • 桜ローファイ // 宵)
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.termguicolors = true
vim.opt.tabstop = 4
vim.opt.shiftwidth = 4
vim.opt.expandtab = false
vim.opt.cursorline = true
vim.opt.signcolumn = "yes"

-- Paleta Anime sincronizada por Kitsune Rice Studio
local colors = {
  bg      = "#181825",
  surface = "#1e1e2e",
  fg      = "#cdd6f4",
  muted   = "#9399b2",
  primary = "#f5c2e7",
  second  = "#cba6f7",
  accent  = "#fab387",
}

vim.api.nvim_set_hl(0, "Normal",       { fg = colors.fg, bg = colors.bg })
vim.api.nvim_set_hl(0, "CursorLine",   { bg = colors.surface })
vim.api.nvim_set_hl(0, "LineNr",       { fg = colors.muted })
vim.api.nvim_set_hl(0, "CursorLineNr", { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "Keyword",      { fg = colors.second, bold = true })
vim.api.nvim_set_hl(0, "Function",     { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "String",       { fg = "#a6e3a1" })
vim.api.nvim_set_hl(0, "Comment",      { fg = colors.muted, italic = true })
vim.api.nvim_set_hl(0, "StatusLine",   { fg = "#090812", bg = colors.primary, bold = true })
