-- Script legado desativado: ele apagava dados reais e aplicava credenciais de demonstração.
-- Planeje uma migração específica e revisada para qualquer limpeza de ambiente.
DO $$ BEGIN RAISE EXCEPTION 'clean_database.sql desativado por segurança'; END $$;
