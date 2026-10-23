-- 🦊 Kitsune Go Wayland — Neovim Go Studio (Midnight Evangelion • 第3新東京市 // NERV)
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
  bg      = "#10081c",
  surface = "#190e2c",
  fg      = "#f0fdf4",
  muted   = "#a78bfa",
  primary = "#39ff14",
  second  = "#ff9e00",
  accent  = "#9d4edd",
}

vim.api.nvim_set_hl(0, "Normal",       { fg = colors.fg, bg = colors.bg })
vim.api.nvim_set_hl(0, "CursorLine",   { bg = colors.surface })
vim.api.nvim_set_hl(0, "LineNr",       { fg = colors.muted })
vim.api.nvim_set_hl(0, "CursorLineNr", { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "Keyword",      { fg = colors.second, bold = true })
vim.api.nvim_set_hl(0, "Function",     { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "String",       { fg = "#39ff14" })
vim.api.nvim_set_hl(0, "Comment",      { fg = colors.muted, italic = true })
vim.api.nvim_set_hl(0, "StatusLine",   { fg = "#090812", bg = colors.primary, bold = true })
