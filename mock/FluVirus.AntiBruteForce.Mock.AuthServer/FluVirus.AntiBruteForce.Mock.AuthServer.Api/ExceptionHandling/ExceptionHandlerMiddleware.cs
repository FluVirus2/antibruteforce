namespace FluVirus.AntiBruteForce.Mock.AuthServer.Api.ExceptionHandling;

public class ExceptionHandlerMiddleware(
    ILogger<ExceptionHandlerMiddleware> logger
) : IMiddleware
{
    public async Task InvokeAsync(HttpContext context, RequestDelegate next)
    {
        try
        {
            await next(context);
        }
        catch (Exception e)
        {
            logger.LogError(exception: e, message: "Occurred exception: {ExceptionType}", e.GetType().FullName);

            context.Response.StatusCode = StatusCodes.Status500InternalServerError;
        }
    }
}
