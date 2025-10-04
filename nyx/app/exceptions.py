class BotNotWorkingException(Exception):
    pass

class PageLoadTimeoutException(Exception):
    pass

class SubmitException(Exception):
    pass

class EntityNotFoundException(Exception):
    pass

class InvalidSolutionException(Exception):
    pass

class ServerStartException(Exception):
    pass

class CorruptedRequestException(Exception):
    pass

class ClientDisconnectedException(Exception):
    pass