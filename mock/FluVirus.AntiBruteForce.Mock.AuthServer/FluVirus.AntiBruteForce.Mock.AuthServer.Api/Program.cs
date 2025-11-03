using FluVirus.AntiBruteForce.Mock.AuthServer.Api.AntiBruteForce;
using FluVirus.AntiBruteForce.Mock.AuthServer.Api.ExceptionHandling;
using FluVirus.AntiBruteForce.Mock.AuthServer.Api.Resources;

namespace FluVirus.AntiBruteForce.Mock.AuthServer.Api;

public class Program
{
    public static void Main(string[] args)
    {
        WebApplicationBuilder builder = WebApplication.CreateBuilder(args);
        builder.Services.AddSingleton<ExceptionHandlerMiddleware>();
        builder.Services.AddSingleton<AntiBruteForceMiddleware>();

        WebApplication app = builder.Build();

        app.UseMiddleware<ExceptionHandlerMiddleware>();
        app.UseMiddleware<AntiBruteForceMiddleware>();

        app.MapGet("/resource", async (HttpContext context) =>
            await context.Response.WriteAsJsonAsync(InMemoryResourceStorage.Default, context.RequestAborted)
        );

        app.MapGet("/echo/{value:int}", async (HttpContext context, int value) =>
            await context.Response.WriteAsJsonAsync(new Resource { Value = value }, context.RequestAborted)
        );

        app.Run();
    }
}
