package com.lingqiao.server.scripts;

public class ScriptsService {
    private final FileScriptRepository repository;

    public ScriptsService(FileScriptRepository repository) {
        this.repository = repository;
    }

    public ScriptListResult list() {
        return repository.list();
    }

    public ScriptModule get(String id) {
        return repository.require(id);
    }

    public ScriptModule active() {
        return repository.requireActive();
    }
}
