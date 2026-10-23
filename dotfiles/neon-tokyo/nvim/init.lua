-- 🦊 Kitsune Go Wayland — Neovim Go Studio (Cyberpunk Neon Tokyo • ネオ東京 // 2099)
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
  bg      = "#0b0916",
  surface = "#131026",
  fg      = "#eef6ff",
  muted   = "#8f9bb8",
  primary = "#00f0ff",
  second  = "#ff2a85",
  accent  = "#bd00ff",
}

vim.api.nvim_set_hl(0, "Normal",       { fg = colors.fg, bg = colors.bg })
vim.api.nvim_set_hl(0, "CursorLine",   { bg = colors.surface })
vim.api.nvim_set_hl(0, "LineNr",       { fg = colors.muted })
vim.api.nvim_set_hl(0, "CursorLineNr", { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "Keyword",      { fg = colors.second, bold = true })
vim.api.nvim_set_hl(0, "Function",     { fg = colors.primary, bold = true })
vim.api.nvim_set_hl(0, "String",       { fg = "#00ffa3" })
vim.api.nvim_set_hl(0, "Comment",      { fg = colors.muted, italic = true })
vim.api.nvim_set_hl(0, "StatusLine",   { fg = "#090812", bg = colors.primary, bold = true })
